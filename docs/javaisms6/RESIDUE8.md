# The T1 / T2 / T3 residue of the eighth flip: a rule or a counted reason for every site

The eighth flip's Java-ism accounting (`$B/runs/jism-flip8/`, made by `docs/javaisms6/rebuild/run.sh` on
`$B/wt/tree8`, flip8-tree cbb1d976; `summary.out`, `verdicts.tsv`) still marks 407 census sites with a rule
that is now on main: RULE T1 83, T2 136, T3 28, T4 63, R4 41, R5 56 (census row 17 is a subset of row 18
and is not added, as in `summary.py`). Those rules are implemented (T1 piece `fns`, T2 kinds `sorted`
`enumset` `cmaps` `deque` `guava`, T3 `goptional` `seqcoll`, "T3" of JAVAISMS6 = piece `objarg`, T4
half = piece `cmp`, R4 `ownwild`, R5 `nbox`; COLLECTIONS-PLAN.md sections 6-12), so each remaining site is
something a rule refused, does not cover, or something the flip tree has only because it predates the
rule. This file gives every site one of three verdicts:

- **gap for a rule**: a further mechanical rule (a twin, a maker, a name list, a typed runtime class; no
  algorithm or data-structure change of a pass) could take it; the rule is described and its sites counted;
- **counted reason**: it stays, for a reason that holds up against how jsgo (`/home/ubuntu/work/jsgo`)
  handled the same Java code: jsgo keeps it `any` too, or types it only with a hand-written structure or
  rewrite that our bounds forbid, or Java itself declares Object (or the site is not a site);
- **taken on main**: the flip tree predates `ownwild` / `nbox`; main's current translation no longer has
  the construct (checked mechanically, below).

## Totals

| rule | sites | gap for a rule | counted reason | taken on main |
|---|---:|---:|---:|---:|
| T1 | 83 | 74 | 7 | 2 |
| T2 | 136 | 105 | 31 | 0 |
| T3 | 28 | 21 | 7 | 0 |
| T4 | 63 | 53 | 10 | 0 |
| R4 | 41 | 0 | 0 | 41 |
| R5 | 56 | 3 | 1 | 52 |
| **all** | **407** | **256** | **56** | **95** |

(T1 + T2 + T3 alone: 247 sites = 200 gap + 45 reason + 2 taken.)

The 256 gap sites, grouped by the rule that would take them (category ids of the tables below):

| proposed rule | categories | sites |
|---|---|---:|
| Remove javac's bridges once their erased callers are typed (typed sorted builders, an `Arrays_sortT_` twin, typed Ordering / TreeRangeSet, the erased `iterator()` readers), then a deadbridge-style proof that no erased call reaches the bridge; the clone bridges are dead already | T4.1, T4.2a, T4.3a | 42 |
| Typed runtime instance methods, by name list: `BigInteger.compareTo_t`, `ComparisonChain.compare_s`, `Enum.compareTo` by ordinal, `BitSet.clone_t`, ImmutableList's own `subList` / `reverse` / `asList` on a typed immutable, `tailSet_t`, `containsValue_t`, `ListT.add_int_t` | T1.1-T1.4, T2.13, T2.16a, T3.3a | 40 |
| Piece makers extended: fixed-arity / array / empty `of` / `copyOf` for a typed place, builder chains Colls does not prove, `new EnumMap / IdentityHashMap / LinkedList / ConcurrentHashMap` for a typed place (then the objarg pairs) | T2.3, T2.4, T2.6, T2.8, T3.2 | 37 |
| Typed Guava sorted makers and builders (`ImmutableSortedSet.copyOf / copyOfSorted / of / naturalOrder()`, `ImmutableSortedMap.copyOf`, `sortedCopyOf(cmp, ..)`, comparator-taking makers) | T2.2, T1.14, T2.19 | 35 |
| More `LibFns` twins: `Arrays.setAll`, `Comparator.comparing` with an ordered key, `Lists.transform` read once, `Splitter.split`, `Io.listFiles`, `Map.putIfAbsent` / `replace`, `Collections.singletonList`, `Iterables.concat` | T1.5-T1.7, T1.9, T1.10, T1.13, T1.15 | 26 |
| Typed runtime classes: `Ordering[T]` as a func type, `ThreadLocal[T]`, `RangeSet[C]`, `LoadingCache` built from a typed `CacheLoader`, `AbstractIteratorT[T]`, `AbstractSetT[T]` | T2.9-T2.11, T1.11, T4.3b, T4.2b, T4.3c | 23 |
| Typed for-each over a fresh copy (`ImmutableList.copyOf / of / sortedCopyOf` read only by the loop) or a reversed list | T2.1, T1.8 | 21 |
| Piece objarg's List / Collection twins again (built in COLLECTIONS-PLAN 10.8 and removed for binary size: the size cost decides) | T3.1 | 16 |
| `Conv.strSeq` name list: String variants of more CharSequence parameters | T1.12, T2.12, T3.4 | 6 |
| `Source.Lazy<T>` typed: a translator defect (a type-annotated qualified type `Source.@Nullable Lazy<String>` reads as raw) plus its equals on `tools/j2go/ownwild-equals.txt` | T1.21, T4.3d, R5.3 | 6 |
| Piece nbox for type arguments (`Function<X, Integer>`, `Map<K, Integer / Long>` hold `Nbox[T]`) | T1.16, R5.4 | 4 |
| **all** | | **256** |

The 56 counted reasons: Java declares Object (T1.18 4, T1.20 1, T2.5 4, T2.17 3, T3.3b 6, T4.3e 3,
T4.3f 1), a key or element the typed family does not hold (T2.7 8, T2.15 7, T4.1b 2), a method type
variable (T2.14 2, T2.18 5, T3.3c 1), an Object nobody reads / an `Iterable<?>` dispatched by instanceof
(T2.16b 2), a descriptor's unbox (T1.19 2), and 5 that are not sites (T4.4 4 placeholder stubs, R5.2 1
census match inside a string). In each of these jsgo either keeps `any` or replaced the Java structure
by hand (split maps, tagged structs, inlined generic methods, bitmasks): the tables cite where.

T4 was the accounting's "bridge" rule; with jsgo's form (typed `CompareTo(*T)` and no bridge) in view,
most of it is a gap, not a reason: a bridge stays only while an erased caller remains, and each such
caller is itself one of the gaps above.

## How it was made (reproducible)

```
docs/javaisms6/rebuild/run.sh $B/wt/tree8/go/closure $B/runs/jism-flip8    # the accounting (existing)
flock $B/heavy.lock systemd-run --user --scope -q -p MemoryMax=20G -p MemorySwapMax=0 env GOENV=$B/goenv-c1 \
  bash docs/javaisms6/rebuild/residue8.sh $B/runs/jism-flip8 OUTDIR       # on branch main-t5-resid
```

`residue8.sh` translates this branch twice: `base/` with `J2GO_NOLIBGEN=ownwild,nbox,proto` (= the
translator the eighth flip was made from, main f47fbe67, md5 of `cat g_*.go` fab9e90c) and `main/` with
`J2GO_NOLIBGEN=proto` (= main 73e902a8's default, 13f040cd); it stops if either md5 differs. Then
`py/residue8.py ACC base main` writes `residue8.tsv` (one line per site) and `residue8.out` (the tables
below). The categories, their verdicts, texts and jsgo citations are data in `py/residue8_cats.py`:

- **Category**: the first entry whose rule, census row, label (classify6's) and source line (matched
  against `file<TAB>line`) match. Every one of the 407 sites matches one (the script fails otherwise).
- **Taken-on-main check**, per site: if the site's g_ file is byte-identical in `base/` and `main/`,
  nothing changed it ("file unchanged"); else its line is found in its enclosing function (else the file)
  of `base/` (exactly, else the closest line, difflib), the function of `main/` is aligned with it, and a
  pattern (the category's, else the census row's: `obj_(` ... for boxing, `cast_[` for casts, `\bany\b` for
  Object slots) is counted in the base line and in the main line it became. Fewer in main, or the line gone:
  "taken on main". Two sites whose line a post-flip step rewrote beyond recognition are checked by hand
  (`MANUAL` in `residue8_cats.py`, with the main line as evidence).
- The jsgo citations come from reading jsgo at the cited lines (three read-only passes over the sites,
  one per rule group); jsgo paths are relative to `/home/ubuntu/work/jsgo`.

Notes on the accounting itself, found on the way: R5's site `g_CommandLineRunner.go:1198` is the word
"any" inside the `--formatting` usage string (not a site); T4's 4 `g_zstubs.go` sites are placeholder
methods of `AbstractMap`, which no Java class of the program names; T4's `computeNext`, CacheLoader
`load`, AbstractSet `add`, Source `code()` and `Function<Integer, Void>.apply` sites are not javac
bridges but erased runtime classes / an erased program generic.

## Tables

(The output of `py/residue8.py`, `residue8.out`, as made by `residue8.sh` on this branch; the per-site lines are also in `docs/javaisms6/residue8.tsv`.)

Sites by rule (the accounting's RULE counts, census row 17 not added): T1 83, T2 136, T3 28, T4 63, R4 41, R5 56

### T1: 83 sites

| category | sites | verdict | reason, or the rule that would take them | jsgo |
|---|---:|---|---|---|
| T1.1 BigInteger.compareTo | 10 | gap for a rule 10 | rule *typed runtime methods*: a name list of runtime instance methods whose Java parameter is a class (BigInteger.compareTo(BigInteger)), with a typed twin `compareTo_t(*BigInteger)` in the runtime; the call passes the pointer, no `obj_` | `internal/dtoa/d_to_a.go:261`, `internal/jscomp/peephole_fold_constants_comparisons.go:88`: math/big `Cmp` on `*big.Int` |
| T1.2 ComparisonChain.compare(Comparable, Comparable) | 12 | gap for a rule 12 | same rule *typed runtime methods*: `ComparisonChain.compare` of two Strings is a `compare_s(string, string)` twin (the overload is chosen by javac's argument types), no `strobj_` | `internal/jscomp/lint_check_requires_sorted.go:415`, `internal/jscomp/serialization_serialize_types_to_pointers.go:327`: an if-chain of typed string / int compares |
| T1.3 Enum.compareTo(E) | 2 | gap for a rule 2 | same rule *typed runtime methods*: `Enum.compareTo(E)` of a program enum compares ordinals (a twin `compareOrd_(a, b)`), no `obj_` | `internal/jscomp/check_access_controls_properties.go:505`, `internal/jscomp/sorting_error_manager.go:258`: int-backed enums compared as ints |
| T1.4 Object.clone() of a BitSet | 6 | gap for a rule 6 | rule *typed runtime clone*: a runtime class whose `clone()` gives its own class (BitSet) has `clone_t() *BitSet`; the translator calls it for `bitSet.clone()` cast to BitSet (a name list of such classes) | `internal/jutil/bitset.go:264`: `Clone() *BitSet` (its own sites use a hand `smallBitSet`) |
| T1.5 Arrays.setAll / toArray of a typed array | 5 | gap for a rule 5 | piece fns, one more twin: `Arrays.setAll(T[], IntFunction<T>)` on an array of a class type is `Arrays_setAllT_[T]([]T, IntFunction[T])`; today the array is copied to `[]any` (`arrmap_`), filled through a boxing lambda and cast back element by element | `internal/jscomp/type_transformation.go:344`: a typed slice filled by a loop (Coalesce / LCA: hand structures) |
| T1.6 Comparator.comparing / thenComparing with a key typed any | 6 | gap for a rule 6 | piece fns: `Comparator_comparing` / `thenComparing` twins with the key at a Go ordered type (`U` a String or a primitive box): the runtime's `comparing` erases `U extends Comparable` to `any`, so the key lambda boxes its result | `internal/jscomp/deps_module_resolver.go:389`, `internal/refactoring/apply_suggested_fixes.go:305`: `slices.SortFunc` with typed string / int keys |
| T1.7 Lists.transform (a live view) | 6 | gap for a rule 6 | piece fns: `Lists.transform(l, f)` whose result is only read once as an Iterable (here the argument of `addAll`) is the twin of `Iterables.transform` (`mapIterT_`, already in the runtime); a twin that is a live List view needs a runtime List class at the element type, which is why `LibFns.REFUSED` lists it | `internal/refactoring/refactoring_driver.go:273`: a loop appending to a typed slice |
| T1.8 for-each over Lists.reverse | 2 | gap for a rule 2 | rule *typed for-each over a reversed list*: `for (x : Lists.reverse(list))` of a typed list runs a typed cursor backwards (`eachRevT_(list)`); no view is made | `internal/jscomp/extern_exports_pass.go:118`: a reverse index loop over `[]string` |
| T1.9 for-each over Splitter.split | 2 | gap for a rule 2 | piece fns, one more twin: `Splitter.split(String)` read by a for-each is `splitT_` (an `IterableT[string]`), as `splitToList` already is (`splitToListST_`) | `internal/jscomp/gather_module_metadata.go:868`: a hand byte scan (algorithm change) |
| T1.10 for-each over File.listFiles (Io_listFiles) | 2 | gap for a rule 2 | piece fns, one more twin: the program helper `Io.listFiles(String)` returns `List<String>`, the runtime's `Io_listFiles` an erased List; a twin giving `ListT[string]` | not ported (searched ProductionInstrumentationReporter, getAllExecutionResults, listFiles); nearest `internal/jutil/file.go:172` `ListFiles() []*File` |
| T1.11 CacheLoader.from / CacheBuilder<Object, Object> | 3 | gap for a rule 3 | kind guava extended: `CacheBuilder<K, V>.build(CacheLoader.from(f))` typed as `LoadingCacheT[K, V]` built from a typed function (a twin `cacheLoaderFromT_[K, V]`); today the loader is a converting closure over `any` and the cache enters through `loadingCacheAs_` | `internal/bundle/cached_transformer.go:32`: hand generic `ObjectLRUCache[*Source, *Source]` |
| T1.12 a String given to a CharSequence parameter of a runtime function | 3 | gap for a rule 3 | `Conv.strSeq` (the String variants of functions with CharSequence parameters) extended by name to `TextFormat.merge`, `Ascii.equalsIgnoreCase`, `Integer.parseInt(CharSequence, int, int, int)` | `internal/jscomp/proto_text_format.go:51` (string parameter), `internal/jscomp/peephole_replace_known_methods.go:581` |
| T1.13 Map.putIfAbsent / Map.replace / Collections.singletonList on typed values | 3 | gap for a rule 3 | piece fns, more twins: `Map.putIfAbsent` and `Map.replace` on a typed map (the receiver is typed, `Map_un_` takes the erased view), `Collections.singletonList(T)` for a typed place | `internal/cli/command_line_flags.go:840` (`[]string{..}`), `internal/jscomp/compiler_sources.go:482` (typed map); no AtomicTracerStatMap in jsgo |
| T1.14 converting closures of typed comparators given to Guava sorted makers | 6 | gap for a rule 6 | with T2.2 (typed sorted makers): `ImmutableList.sortedCopyOf(cmp, ..)`, `new ImmutableSortedSet.Builder(cmp)`, `ImmutableSortedSet.copyOf(cmp, ..)` with twins taking `Comparator[T]`; today the typed comparator is wrapped in a closure over `any` | `internal/jscomp/ijs_convert_to_typed_interface.go:716`, `internal/jscomp/deps_module_loader.go:269`: `slices.SortStableFunc` with a typed comparator; `internal/jscomp/variable_map.go:199` sorts keys (VariableMap's elements are Map.Entry: needs typed entries too) |
| T1.15 Iterables.concat of an Iterables.transform | 2 | gap for a rule 2 | piece fns: `Iterables.concat(Iterable<? extends Iterable<T>>)` twin (`concatIterT_`), so `Iterables.transform(chunks, JSChunk::getInputs)` (a function to a List, refused today: "a parameter's Go type is not the twin's") can be typed | `internal/jscomp/js_chunk_graph.go:267`: an eager loop into `[]*CompilerInput` |
| T1.16 function results that are a boxed Integer | 3 | gap for a rule 3 | piece nbox extended to type arguments: `Function<X, Integer>`, `Map<Integer, V>` / `Map<K, Integer>` hold `Nbox[int32]` where Go has `any` (rule "function types" makes a boxed type argument `any`, FnTypes.norm) | `internal/jscomp/serialization_serialize_types_to_pointers.go:35` `map[rhino.JSType]int`, `internal/jscomp/serialization_color_serializer.go:105` `func(string) int`, `internal/jscomp/type_inference_pass.go:41` |
| T1.17 option setters Consumer<Boolean> (nbox) | 2 | taken on main 2 | left by piece nbox (none expected: see the check) | - |
| T1.18 Supplier<Object> / a map value that is Object in Java | 4 | counted reason 4 | the Java type is Object: `logForDiagnostics(String, Supplier<Object>)`, `getPropWithDefault(Property<T>, Supplier<T>)` with T erased in the props map of `Object` values, `ImmutableMap<Object, AssertionFunctionSpec>`; a value meets Object as Java has it | `internal/diagnostic/log_file.go:311` (`any` sink), `internal/rhino/js_doc_info_builder.go:747` (`GetProp` gives `any`), `internal/jscomp/coding_convention.go:662` (map split by hand) |
| T1.19 converting closure of a func-typed element (descriptor) | 2 | counted reason 2 | the `unbox` of the descriptor of a typed collection whose elements are functions: it runs only where erased code reads such a collection; JAVAISMS6 section 3 says the descriptors' unbox fields are not sites | none: jsgo replaced the function-valued collections by hand code (`internal/jscomp/type_reverse_abstract_interpreter.go:182`) |
| T1.20 a Predicate over the edge type Object | 1 | counted reason 1 | `LinkedDirectedGraph<ColorGraphNode, Object>`: the edge type is Object in Java, so `Predicate<E>` is `Predicate<Object>`; jsgo typed it only by narrowing the declared Object to its edge enum and adding a graph method (whole-program narrowing plus an API change) | `internal/jscomp/disambiguate_color_graph_builder.go:53`, `internal/graph/linked_directed_graph.go:318`: E narrowed by hand, a new graph method |
| T1.21 Supplier<String> given to Source.Builder (converting closure) | 1 | gap for a rule 1 | the program generic `Source.Lazy<T>` is erased (`Source_Lazy_memoize(Supplier_base[any])`), so the typed `Supplier<String>` is wrapped; `Lazy` is only instantiated at String: typing `Lazy<T>` (a Go generic, or a name list specializing it to String) removes the wrap | `internal/bundle/source.go:35`: `Lazy` specialized by hand to `lazyString` |

T1 total 83: gap for a rule 74, counted reason 7, taken on main 2

<details><summary>sites</summary>

| site (eighth flip tree) | function | census row | category | verdict | check (base -> main) |
|---|---|---:|---|---|---|
| g_0casts.go:1719 |  | 11 | T1.19 converting closure of a func-typed element (descriptor) | counted reason | closest line; pattern 0 -> 0 |
| g_0casts.go:2045 |  | 16 | T1.19 converting closure of a func-typed element (descriptor) | counted reason | closest line; pattern 1 -> 1 |
| g_BrowserWithTransformedPrefixesModuleResolver.go:35 | BrowserWithTransformedPrefixesModuleResolver.init_BrowserWithTransformedPrefixesModuleResolver | 15 | T1.6 Comparator.comparing / thenComparing with a key typed any | gap for a rule | file unchanged |
| g_CachedTransformer.go:10 | CachedTransformer.init_CachedTransformer_Transformer_CacheBuilder | 11 | T1.11 CacheLoader.from / CacheBuilder<Object, Object> | gap for a rule | file unchanged |
| g_CachedTransformer.go:10 | CachedTransformer.init_CachedTransformer_Transformer_CacheBuilder | 15 | T1.11 CacheLoader.from / CacheBuilder<Object, Object> | gap for a rule | file unchanged |
| g_CheckAccessControls.go:489 | CheckAccessControls_canOverrideVisibility | 15 | T1.3 Enum.compareTo(E) | gap for a rule | file unchanged |
| g_CheckRequiresSorted.go:403 | CheckRequiresSorted_DestructuringBinding.compareTo_t | 15 | T1.2 ComparisonChain.compare(Comparable, Comparable) | gap for a rule | file unchanged |
| g_CheckRequiresSorted.go:403 | CheckRequiresSorted_DestructuringBinding.compareTo_t | 15 | T1.2 ComparisonChain.compare(Comparable, Comparable) | gap for a rule | file unchanged |
| g_CheckRequiresSorted.go:403 | CheckRequiresSorted_DestructuringBinding.compareTo_t | 15 | T1.2 ComparisonChain.compare(Comparable, Comparable) | gap for a rule | file unchanged |
| g_CheckRequiresSorted.go:403 | CheckRequiresSorted_DestructuringBinding.compareTo_t | 15 | T1.2 ComparisonChain.compare(Comparable, Comparable) | gap for a rule | file unchanged |
| g_CoalesceVariableNames.go:180 | CoalesceVariableNames.computeVariableNamesInterferenceGraph | 11 | T1.5 Arrays.setAll / toArray of a typed array | gap for a rule | file unchanged |
| g_CoalesceVariableNames.go:177 | CoalesceVariableNames.computeVariableNamesInterferenceGraph | 15 | T1.5 Arrays.setAll / toArray of a typed array | gap for a rule | file unchanged |
| g_CodeReplacement.go:108 | CodeReplacement_clinit | 15 | T1.6 Comparator.comparing / thenComparing with a key typed any | gap for a rule | file unchanged |
| g_CodeReplacement.go:108 | CodeReplacement_clinit | 15 | T1.6 Comparator.comparing / thenComparing with a key typed any | gap for a rule | file unchanged |
| g_CodingConvention.go:474 | CodingConvention_AssertionFunctionLookup.lookupByCallee | 11 | T1.18 Supplier<Object> / a map value that is Object in Java | counted reason | file unchanged |
| g_CommandLineRunner.go:569 | CommandLineRunner_loadConformanceConfig | 15 | T1.12 a String given to a CharSequence parameter of a runtime function | gap for a rule | same line; pattern 1 -> 1 |
| g_CommandLineRunner.go:1588 | CommandLineRunner_Flags.getMixedJsSources | 15 | T1.13 Map.putIfAbsent / Map.replace / Collections.singletonList on typed values | gap for a rule | same line; pattern 1 -> 1 |
| g_Compiler.go:2406 | Compiler.getSourceMapping_Optional_int_int_impl | 15 | T1.13 Map.putIfAbsent / Map.replace / Collections.singletonList on typed values | gap for a rule | file unchanged |
| g_ConvertToTypedInterface.go:570 | ConvertToTypedInterface_SimplifyDeclarations_clinit | 15 | T1.6 Comparator.comparing / thenComparing with a key typed any | gap for a rule | file unchanged |
| g_DToA.go:637 | DToA_JS_dtoa | 15 | T1.1 BigInteger.compareTo | gap for a rule | file unchanged |
| g_DToA.go:650 | DToA_JS_dtoa | 15 | T1.1 BigInteger.compareTo | gap for a rule | file unchanged |
| g_DToA.go:677 | DToA_JS_dtoa | 15 | T1.1 BigInteger.compareTo | gap for a rule | file unchanged |
| g_DToA.go:683 | DToA_JS_dtoa | 15 | T1.1 BigInteger.compareTo | gap for a rule | file unchanged |
| g_DToA.go:703 | DToA_JS_dtoa | 15 | T1.1 BigInteger.compareTo | gap for a rule | file unchanged |
| g_DToA.go:763 | DToA_JS_dtoa | 15 | T1.1 BigInteger.compareTo | gap for a rule | file unchanged |
| g_DisambiguateProperties.go:69 | DisambiguateProperties.process_2 | 15 | T1.18 Supplier<Object> / a map value that is Object in Java | counted reason | file unchanged |
| g_DisambiguateProperties.go:92 | DisambiguateProperties.process_2 | 15 | T1.18 Supplier<Object> / a map value that is Object in Java | counted reason | file unchanged |
| g_DisambiguateProperties.go:350 | DisambiguateProperties_PropertyReferenceJson.compareTo_t | 15 | T1.2 ComparisonChain.compare(Comparable, Comparable) | gap for a rule | file unchanged |
| g_DisambiguateProperties.go:350 | DisambiguateProperties_PropertyReferenceJson.compareTo_t | 15 | T1.2 ComparisonChain.compare(Comparable, Comparable) | gap for a rule | file unchanged |
| g_DotFormatter.go:90 | DotFormatter_toDot_GraphvizGraph | 15 | T1.6 Comparator.comparing / thenComparing with a key typed any | gap for a rule | file unchanged |
| g_ExternExportsPass.go:526 | ExternExportsPass_PropertyExport.getExportedPath_impl | 5 | T1.8 for-each over Lists.reverse | gap for a rule | file unchanged |
| g_ExternExportsPass.go:64 | ExternExportsPass.process_2 | 15 | T1.6 Comparator.comparing / thenComparing with a key typed any | gap for a rule | file unchanged |
| g_ExternExportsPass.go:529 | ExternExportsPass_PropertyExport.getExportedPath_impl | 16 | T1.8 for-each over Lists.reverse | gap for a rule | file unchanged |
| g_GatherModuleMetadata.go:43 | GatherModuleMetadata_isValidModuleId | 5 | T1.9 for-each over Splitter.split | gap for a rule | file unchanged |
| g_GatherModuleMetadata.go:45 | GatherModuleMetadata_isValidModuleId | 16 | T1.9 for-each over Splitter.split | gap for a rule | file unchanged |
| g_JSChunkGraph.go:160 | JSChunkGraph.getAllInputs | 13 | T1.15 Iterables.concat of an Iterables.transform | gap for a rule | file unchanged |
| g_JSDocInfo.go:2864 | JSDocInfo_Builder.putPropEntry | 15 | T1.18 Supplier<Object> / a map value that is Object in Java | counted reason | same line; pattern 1 -> 1 |
| g_LiveVariablesAnalysis.go:297 | LiveVariablesAnalysis_LiveVariableLattice_createFromLattice | 11 | T1.4 Object.clone() of a BitSet | gap for a rule | file unchanged |
| g_LiveVariablesAnalysis.go:297 | LiveVariablesAnalysis_LiveVariableLattice_createFromLattice | 15 | T1.4 Object.clone() of a BitSet | gap for a rule | file unchanged |
| g_LowestCommonAncestorFinder.go:168 | LowestCommonAncestorFinder_Color_clinit | 11 | T1.5 Arrays.setAll / toArray of a typed array | gap for a rule | file unchanged |
| g_LowestCommonAncestorFinder.go:166 | LowestCommonAncestorFinder_Color_clinit | 15 | T1.5 Arrays.setAll / toArray of a typed array | gap for a rule | file unchanged |
| g_OptimizeParameters.go:744 | OptimizeParameters_UnusedParameterOptimizer.tryEliminateUnusedArgs | 11 | T1.4 Object.clone() of a BitSet | gap for a rule | file unchanged |
| g_OptimizeParameters.go:749 | OptimizeParameters_UnusedParameterOptimizer.tryEliminateUnusedArgs | 11 | T1.4 Object.clone() of a BitSet | gap for a rule | file unchanged |
| g_OptimizeParameters.go:744 | OptimizeParameters_UnusedParameterOptimizer.tryEliminateUnusedArgs | 15 | T1.4 Object.clone() of a BitSet | gap for a rule | file unchanged |
| g_OptimizeParameters.go:749 | OptimizeParameters_UnusedParameterOptimizer.tryEliminateUnusedArgs | 15 | T1.4 Object.clone() of a BitSet | gap for a rule | file unchanged |
| g_PeepholeFoldConstants.go:1038 | PeepholeFoldConstants_tryAbstractRelationalComparison | 15 | T1.1 BigInteger.compareTo | gap for a rule | file unchanged |
| g_PeepholeFoldConstants.go:1052 | PeepholeFoldConstants_tryAbstractRelationalComparison | 15 | T1.1 BigInteger.compareTo | gap for a rule | file unchanged |
| g_PeepholeFoldConstants.go:1060 | PeepholeFoldConstants_tryAbstractRelationalComparison | 15 | T1.1 BigInteger.compareTo | gap for a rule | file unchanged |
| g_PeepholeFoldConstants.go:1096 | PeepholeFoldConstants_bigintLessThanDouble | 15 | T1.1 BigInteger.compareTo | gap for a rule | file unchanged |
| g_PeepholeReplaceKnownMethods.go:515 | PeepholeReplaceKnownMethods.tryFoldParseNumber | 15 | T1.12 a String given to a CharSequence parameter of a runtime function | gap for a rule | file unchanged |
| g_ProductionInstrumentationReporter.go:47 | ProductionInstrumentationReporter.getAllExecutionResults | 5 | T1.10 for-each over File.listFiles (Io_listFiles) | gap for a rule | file unchanged |
| g_ProductionInstrumentationReporter.go:49 | ProductionInstrumentationReporter.getAllExecutionResults | 16 | T1.10 for-each over File.listFiles (Io_listFiles) | gap for a rule | file unchanged |
| g_RefactoringDriver.go:96 | RefactoringDriver_Builder.addExternsFromFile | 15 | T1.7 Lists.transform (a live view) | gap for a rule | file unchanged |
| g_RefactoringDriver.go:122 | RefactoringDriver_Builder.addInputsFromFile | 15 | T1.7 Lists.transform (a live view) | gap for a rule | file unchanged |
| g_RefactoringDriver.go:96 | RefactoringDriver_Builder.addExternsFromFile | 16 | T1.7 Lists.transform (a live view) | gap for a rule | file unchanged |
| g_RefactoringDriver.go:122 | RefactoringDriver_Builder.addInputsFromFile | 16 | T1.7 Lists.transform (a live view) | gap for a rule | file unchanged |
| g_SerializeTypesToPointers.go:152 | SerializeTypesToPointers_TypeSearchCallback.visit | 15 | T1.16 function results that are a boxed Integer | gap for a rule | file unchanged |
| g_SerializeTypesToPointers.go:251 | SerializeTypesToPointers_TypeMismatchJson.compareTo_t | 15 | T1.2 ComparisonChain.compare(Comparable, Comparable) | gap for a rule | file unchanged |
| g_SerializeTypesToPointers.go:251 | SerializeTypesToPointers_TypeMismatchJson.compareTo_t | 15 | T1.2 ComparisonChain.compare(Comparable, Comparable) | gap for a rule | file unchanged |
| g_SerializeTypesToPointers.go:251 | SerializeTypesToPointers_TypeMismatchJson.compareTo_t | 15 | T1.2 ComparisonChain.compare(Comparable, Comparable) | gap for a rule | file unchanged |
| g_SerializeTypesToPointers.go:251 | SerializeTypesToPointers_TypeMismatchJson.compareTo_t | 15 | T1.2 ComparisonChain.compare(Comparable, Comparable) | gap for a rule | file unchanged |
| g_SerializeTypesToPointers.go:251 | SerializeTypesToPointers_TypeMismatchJson.compareTo_t | 15 | T1.2 ComparisonChain.compare(Comparable, Comparable) | gap for a rule | file unchanged |
| g_SerializeTypesToPointers.go:251 | SerializeTypesToPointers_TypeMismatchJson.compareTo_t | 15 | T1.2 ComparisonChain.compare(Comparable, Comparable) | gap for a rule | file unchanged |
| g_SortingErrorManager.go:179 | SortingErrorManager_LeveledJSErrorComparator.compare | 15 | T1.3 Enum.compareTo(E) | gap for a rule | file unchanged |
| g_Source.go:230 | Source_Builder.setCodeFromSupplier | 15 | T1.21 Supplier<String> given to Source.Builder (converting closure) | gap for a rule | file unchanged |
| g_SourceMapJsonLexer.go:314 | SourceMapJsonLexer_unescape | 15 | T1.12 a String given to a CharSequence parameter of a runtime function | gap for a rule | file unchanged |
| g_Tracer.go:631 | Tracer_AtomicTracerStatMap.incrementBy | 15 | T1.13 Map.putIfAbsent / Map.replace / Collections.singletonList on typed values | gap for a rule | file unchanged |
| g_TypeTransformation.go:271 | TypeTransformation.evalTemplatizedType | 11 | T1.5 Arrays.setAll / toArray of a typed array | gap for a rule | file unchanged |
| g_TypedAstSerializer.go:511 | TypedAstSerializer.createColorTypeSerializer | 15 | T1.16 function results that are a boxed Integer | gap for a rule | file unchanged |
| g_CachedTransformer.go:10 | CachedTransformer.init_CachedTransformer_Transformer_CacheBuilder | 100 | T1.11 CacheLoader.from / CacheBuilder<Object, Object> | gap for a rule | file unchanged |
| g_ColorGraphBuilder.go:105 | ColorGraphBuilder.connectSourceToDest | 100 | T1.20 a Predicate over the edge type Object | counted reason | file unchanged |
| g_ConvertToTypedInterface.go:419 | ConvertToTypedInterface_SimplifyDeclarations.simplifyAll | 100 | T1.14 converting closures of typed comparators given to Guava sorted makers | gap for a rule | file unchanged |
| g_ConvertToTypedInterface.go:419 | ConvertToTypedInterface_SimplifyDeclarations.simplifyAll | 100 | T1.14 converting closures of typed comparators given to Guava sorted makers | gap for a rule | file unchanged |
| g_JSChunkGraph.go:160 | JSChunkGraph.getAllInputs | 100 | T1.15 Iterables.concat of an Iterables.transform | gap for a rule | file unchanged |
| g_LinterMain.go:40 | LinterMain.run | 100 | T1.17 option setters Consumer<Boolean> (nbox) | taken on main | same line; pattern 1 -> 0 |
| g_MapDiagnosticToSuppression.go:42 | MapDiagnosticToSuppression.printSuppressions | 100 | T1.17 option setters Consumer<Boolean> (nbox) | taken on main | same line; pattern 1 -> 0 |
| g_ModuleLoader.go:79 | ModuleLoader.createRootPaths | 100 | T1.14 converting closures of typed comparators given to Guava sorted makers | gap for a rule | file unchanged |
| g_ModuleLoader.go:79 | ModuleLoader.createRootPaths | 100 | T1.14 converting closures of typed comparators given to Guava sorted makers | gap for a rule | file unchanged |
| g_RefactoringDriver.go:96 | RefactoringDriver_Builder.addExternsFromFile | 100 | T1.7 Lists.transform (a live view) | gap for a rule | file unchanged |
| g_RefactoringDriver.go:122 | RefactoringDriver_Builder.addInputsFromFile | 100 | T1.7 Lists.transform (a live view) | gap for a rule | file unchanged |
| g_TypeInferencePass.go:94 | TypeInferencePass.inferScope | 100 | T1.16 function results that are a boxed Integer | gap for a rule | file unchanged |
| g_VariableMap.go:58 | VariableMap.toBytes | 100 | T1.14 converting closures of typed comparators given to Guava sorted makers | gap for a rule | file unchanged |
| g_VariableMap.go:58 | VariableMap.toBytes | 100 | T1.14 converting closures of typed comparators given to Guava sorted makers | gap for a rule | file unchanged |

</details>

### T2: 136 sites

| category | sites | verdict | reason, or the rule that would take them | jsgo |
|---|---:|---|---|---|
| T2.17 an Object parameter or an Object value: no typed place | 3 | counted reason 3 | the value goes to an Object: `checkState(boolean, Object)`'s message argument (CheckSuper), `(Iterable<?>) value` of an Object (LogsGson), the result of a `Supplier<Object>` (DisambiguateProperties) | `internal/jscomp/check_super.go:151` (a formatted string), `internal/diagnostic/log_file.go:311` (`any`) |
| T2.18 a generic method's type variable | 5 | counted reason 5 | `<T> concat(..)` (CompilerInput) and the generic `addNewEntry` (TypeTransformation): the method type variable is Object in Go (as T2.14); jsgo duplicated the method per type by hand | `internal/jscomp/compiler_input.go:714`, `internal/jscomp/type_transformation.go:94`: duplicated per type by hand |
| T2.5 ImmutableMap<Object, ..>: a key that is Object in Java | 4 | counted reason 4 | `ImmutableMap<Object, AssertionFunctionSpec>`: the key is Object in Java (`getId()` returns Object: a ClosurePrimitive or a String); jsgo split the map by hand into two typed maps | `internal/jscomp/coding_convention.go:604`: split by hand into two typed maps |
| T2.15 ImmutableMap<ImmutableSet<String>, String> (a collection as a key) | 7 | counted reason 7 | the key is a collection (`ImmutableSet<String>`), which the typed family does not hold as a key (piece elems holds collections as elements and values, not as keys: their equals is by contents); jsgo replaced the map by a hand list of structs | `internal/rhino/js_doc_info.go:63`, `internal/jscomp/js_doc_info_printer.go:163`: a hand list of structs |
| T2.16a TreeSet.tailSet / Map.containsValue on a typed value | 2 | gap for a rule 2 | twins on a name list: `tailSet_t(e, inclusive)` on the typed sorted set (piece cls, as T2.13) and `containsValue_t(V)` on a typed map | `internal/jscomp/lint_check_es6_module_file_structure.go:112` (hand bitset), `internal/jscomp/find_module_dependencies.go:166` (typed loop) |
| T2.16b an Object value nobody reads typed / an Iterable<?> dispatched by instanceof | 2 | counted reason 2 | `List<ListenableFuture<?>>` of futures whose value is never read (PrebuildAst; jsgo replaced the executor by a hand parallel scheme), and `appendAll(Iterable<?>)` whose elements are JSType or String by instanceof (TypeStringBuilder; jsgo split the method by hand) | `internal/jscomp/prebuild_ast_parallel.go:54` (hand parallel scheme), `internal/rhino/jstype_helpers.go:587` (method split by hand) |
| T2.19 typed comparator wrapped for ImmutableList.sortedCopyOf | 2 | gap for a rule 2 | as T1.14: `ImmutableList.sortedCopyOf(Comparator<String>, ..)` with a twin taking `Comparator[T]` | `internal/jscomp/ijs_convert_to_typed_interface.go:716`: `slices.SortStableFunc` with a typed comparator |
| T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | 19 | gap for a rule 19 | rule *typed for-each over a fresh copy* (COLLECTIONS-PLAN 7.6, not built): `for (X x : ImmutableList.copyOf(c))` / `ImmutableList.of(a, b)` read only by the loop makes the typed copy (`immutableListCopyOf_`, a `[]X`) and runs `eachOf_`; the loop variable gives the type | `internal/jscomp/symbol_table.go:910`, `internal/jscomp/compiler_parse.go:494`, `internal/jscomp/closure_rewrite_module.go:706`, `internal/jscomp/serialization_js_type_reconserializer.go:891`: typed slice + loop |
| T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | 27 | gap for a rule 27 | kind sorted extended with typed makers (COLLECTIONS-PLAN 7.6: a typed `ImmutableSortedSet` exists, `tsetImmNew_` of an `icSorted_` twin; the builders need Colls' proof as the other builders) | `internal/refactoring/apply_suggested_fixes.go:209`, `internal/rhino/jstype_property_map.go:479`, `internal/jscomp/variable_map.go:198`: typed slice sorted with `slices.SortStableFunc` |
| T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place | 18 | gap for a rule 18 | piece makers: the fixed-arity and array overloads (`of(e1, e2, ..)`, `copyOf(E[])`) for a typed place make the typed immutable (the runtime has the erased varargs function only, so each element is boxed and the result enters through `tsetImm_` / `mapAs_`) | `internal/jscomp/ast_validator_expr.go:176`, `internal/parser/feature_set.go:747`, `internal/jscomp/compiler_options_enums.go:100`: typed slices / maps |
| T2.4 builders whose chain Colls does not type | 7 | gap for a rule 7 | Colls' builder proof extended: a builder given to `Optional.of`, filled by `addAll` of an erased value, or built and handed to an erased parameter; each builder here is local and only filled and built | `internal/jscomp/closure_coding_convention.go:318`, `internal/bundle/source.go:129`, `internal/jscomp/default_pass_config_factories.go:162`: typed slices and sets |
| T2.6 immutable copies into a typed place | 5 | gap for a rule 5 | piece makers: `copyOf` / `of()` whose result goes to a typed place through an erased path (an `ImmutableList_as_Iterable`, `Lists.reverse`, a slice): the typed maker for that place (`immutableListCopyOf_`, the typed empty) | `internal/jscomp/typed_scope_creator.go:433`, `internal/refactoring/require_alias_generator.go:87`: typed slice, `slices.Reverse` |
| T2.7 TreeMap<Property<?>, Object> of JSDocInfo | 8 | counted reason 8 | COLLECTIONS-PLAN 7.6: the key `JSDocInfo.Property<?>` has type arguments, which the typed family does not hold, and the values are Object (each property its own type) | `internal/rhino/js_doc_info.go:66`: values `any`, a hand bitmask and slice |
| T2.8 EnumMap / IdentityHashMap / LinkedList made for a typed place | 5 | gap for a rule 5 | kind cmaps / deque with makers: `new EnumMap<>(K.class)`, `new IdentityHashMap<>()`, `new LinkedList<>()` for a typed place make the typed map / deque (`TMap` with identity keys for IdentityHashMap, ordinal order for EnumMap) instead of the erased object behind `mapAs_` / `dequeAs_` | `internal/jscomp/check_access_controls.go:64`, `internal/parser/keywords.go:134`, `internal/jscomp/scope.go:48`, `internal/rhino/jstype_template_type_replacer.go:67`: typed slice / ordinal arrays / `map[JSType]JSType` |
| T2.9 Ordering<T> fields | 8 | gap for a rule 8 | rule "function types" for Guava's `Ordering<T>` (a Comparator class: `Ordering_natural`, `nullsFirst`, `compare`): `Ordering[T]` as a func type like `Comparator[T]` | `internal/jscomp/symbol_table.go:64`: typed `func(a, b T) int` fields |
| T2.10 ThreadLocal<T> | 2 | gap for a rule 2 | a typed `ThreadLocal[T]` (two methods, `get` / `set`) as kind guava has `LoadingCacheT` | `internal/jscomp/tracer.go:551`: hand map per thread plus a mutex |
| T2.11 RangeSet<LineAndColumn> / Range.closed | 2 | gap for a rule 2 | a typed `RangeSet[C]` / `Range[C]` over a `ComparableT[C]` (piece cmp gives LineAndColumn `compareTo_t`) | `internal/parsing/ir_factory.go:607`: a hand sorted span slice (a structure our bounds forbid; the element type is typed) |
| T2.12 CharMatcher / Splitter with a String argument | 2 | gap for a rule 2 | `Conv.strSeq` extended by name to `CharMatcher.lastIndexIn` / `collapseFrom` (CharSequence parameters) | `internal/jscomp/deps_path_util.go:88`: string functions |
| T2.13 subList / reverse / asList of an immutable typed list | 6 | gap for a rule 6 | piece cls: ImmutableList's own `subList`, `reverse` and ImmutableSet's `asList` on a typed immutable give the typed immutable (a twin on `TList` / `TSet`), not the erased Guava object that is cast back | `internal/rhino/jstype_js_type_methods.go:798`, `internal/refactoring/suggested_fix.go:192`: typed sub-slices |
| T2.14 Map<V, Event<V>> of a generic method (erased type variable) | 2 | counted reason 2 | piece wide: `<V> void addEvent(V value, Map<V, Event<V>> eventsByKey)`: V is a method type variable that the translator erases (Object in Go); a typed place would need V as a Go type parameter of a method, which Go does not have | `internal/jscomp/timeline.go:101`: addEvent inlined and duplicated by hand per key type |

T2 total 136: gap for a rule 105, counted reason 31

<details><summary>sites</summary>

| site (eighth flip tree) | function | census row | category | verdict | check (base -> main) |
|---|---|---:|---|---|---|
| g_ApplySuggestedFixes.go:101 | ApplySuggestedFixes_applyCodeReplacements | 2 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_AstValidator.go:311 | AstValidator.validatePseudoExpression | 2 | T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place | gap for a rule | file unchanged |
| g_CheckAccessControls.go:16 | CheckAccessControls.finit_CheckAccessControls | 0 | T2.8 EnumMap / IdentityHashMap / LinkedList made for a typed place | gap for a rule | file unchanged |
| g_CheckEs6ModuleFileStructure.go:63 | CheckEs6ModuleFileStructure.checkOrder | 15 | T2.16a TreeSet.tailSet / Map.containsValue on a typed value | gap for a rule | file unchanged |
| g_CheckSuper.go:27 | CheckSuper.process_2 | 2 | T2.17 an Object parameter or an Object value: no typed place | counted reason | file unchanged |
| g_ClosureCodingConvention.go:227 | ClosureCodingConvention.getAssertionFunctions_rImmutableSet | 2 | T2.4 builders whose chain Colls does not type | gap for a rule | file unchanged |
| g_ClosureRewriteModule.go:80 | ClosureRewriteModule.process_2 | 2 | T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | gap for a rule | file unchanged |
| g_ClosureRewriteModule.go:80 | ClosureRewriteModule.process_2 | 15 | T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | gap for a rule | file unchanged |
| g_ClosureRewriteModule.go:80 | ClosureRewriteModule.process_2 | 15 | T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | gap for a rule | file unchanged |
| g_ClosureRewriteModule.go:81 | ClosureRewriteModule.process_2 | 16 | T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | gap for a rule | file unchanged |
| g_CodingConvention.go:457 | CodingConvention_AssertionFunctionLookup_of | 2 | T2.5 ImmutableMap<Object, ..>: a key that is Object in Java | counted reason | file unchanged |
| g_CodingConvention.go:468 | CodingConvention_AssertionFunctionLookup.lookupByCallee | 11 | T2.5 ImmutableMap<Object, ..>: a key that is Object in Java | counted reason | file unchanged |
| g_CodingConvention.go:459 | CodingConvention_AssertionFunctionLookup_of | 15 | T2.5 ImmutableMap<Object, ..>: a key that is Object in Java | counted reason | file unchanged |
| g_CodingConvention.go:468 | CodingConvention_AssertionFunctionLookup.lookupByCallee | 15 | T2.5 ImmutableMap<Object, ..>: a key that is Object in Java | counted reason | file unchanged |
| g_Compiler.go:490 | Compiler.initBasedOnOptions | 2 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_Compiler.go:1055 | Compiler.getCurrentJsSource | 2 | T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | gap for a rule | file unchanged |
| g_Compiler.go:1737 | Compiler.hoistNoCompileFiles | 2 | T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | gap for a rule | file unchanged |
| g_Compiler.go:3231 | Compiler.resetAndIntitializeSourceMap | 2 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_Compiler.go:1738 | Compiler.hoistNoCompileFiles | 11 | T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | gap for a rule | file unchanged |
| g_Compiler.go:1055 | Compiler.getCurrentJsSource | 15 | T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | gap for a rule | file unchanged |
| g_Compiler.go:1055 | Compiler.getCurrentJsSource | 15 | T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | gap for a rule | file unchanged |
| g_Compiler.go:1056 | Compiler.getCurrentJsSource | 16 | T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | gap for a rule | file unchanged |
| g_CompilerInput.go:157 | CompilerInput.getKnownRequires | 2 | T2.18 a generic method's type variable | counted reason | file unchanged |
| g_CompilerInput.go:182 | CompilerInput.getKnownProvides | 2 | T2.18 a generic method's type variable | counted reason | file unchanged |
| g_CompilerInput.go:323 | CompilerInput_concat | 2 | T2.18 a generic method's type variable | counted reason | file unchanged |
| g_CompilerOptions.go:2785 | CompilerOptions_BrowserFeaturesetYear_clinit | 2 | T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place | gap for a rule | same line; pattern 1 -> 1 |
| g_CompilerOptions.go:2785 | CompilerOptions_BrowserFeaturesetYear_clinit | 15 | T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place | gap for a rule | same line; pattern 10 -> 10 |
| g_CompilerOptions.go:2785 | CompilerOptions_BrowserFeaturesetYear_clinit | 15 | T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place | gap for a rule | same line; pattern 10 -> 10 |
| g_CompilerOptions.go:2785 | CompilerOptions_BrowserFeaturesetYear_clinit | 15 | T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place | gap for a rule | same line; pattern 10 -> 10 |
| g_CompilerOptions.go:2785 | CompilerOptions_BrowserFeaturesetYear_clinit | 15 | T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place | gap for a rule | same line; pattern 10 -> 10 |
| g_CompilerOptions.go:2785 | CompilerOptions_BrowserFeaturesetYear_clinit | 15 | T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place | gap for a rule | same line; pattern 10 -> 10 |
| g_CompilerOptions.go:2785 | CompilerOptions_BrowserFeaturesetYear_clinit | 15 | T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place | gap for a rule | same line; pattern 10 -> 10 |
| g_CompilerOptions.go:2785 | CompilerOptions_BrowserFeaturesetYear_clinit | 15 | T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place | gap for a rule | same line; pattern 10 -> 10 |
| g_CompilerOptions.go:2785 | CompilerOptions_BrowserFeaturesetYear_clinit | 15 | T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place | gap for a rule | same line; pattern 10 -> 10 |
| g_CompilerOptions.go:2785 | CompilerOptions_BrowserFeaturesetYear_clinit | 15 | T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place | gap for a rule | same line; pattern 10 -> 10 |
| g_CompilerOptions.go:2785 | CompilerOptions_BrowserFeaturesetYear_clinit | 15 | T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place | gap for a rule | same line; pattern 10 -> 10 |
| g_ConformanceRules.go:3253 | ConformanceRules_BanCreateDom.initBanCreateDom | 2 | T2.4 builders whose chain Colls does not type | gap for a rule | file unchanged |
| g_ConformanceRules.go:3291 | ConformanceRules_BanCreateDom.checkConformance_impl | 11 | T2.4 builders whose chain Colls does not type | gap for a rule | file unchanged |
| g_ConvertToTypedInterface.go:419 | ConvertToTypedInterface_SimplifyDeclarations.simplifyAll | 16 | T2.19 typed comparator wrapped for ImmutableList.sortedCopyOf | gap for a rule | file unchanged |
| g_ConvertToTypedInterface.go:419 | ConvertToTypedInterface_SimplifyDeclarations.simplifyAll | 16 | T2.19 typed comparator wrapped for ImmutableList.sortedCopyOf | gap for a rule | file unchanged |
| g_DefaultPassConfig.go:1445 | DefaultPassConfig_3.process_2 | 2 | T2.4 builders whose chain Colls does not type | gap for a rule | file unchanged |
| g_DefaultPassConfig.go:1451 | DefaultPassConfig_3.process_2 | 15 | T2.4 builders whose chain Colls does not type | gap for a rule | file unchanged |
| g_DiagnosticToSuppressionMapper.go:51 | DiagnosticToSuppressionMapper.createSuppressionMap | 15 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_DiagnosticToSuppressionMapper.go:51 | DiagnosticToSuppressionMapper.createSuppressionMap | 15 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_DisambiguateProperties.go:69 | DisambiguateProperties.process_2 | 2 | T2.17 an Object parameter or an Object value: no typed place | counted reason | file unchanged |
| g_DisambiguateProperties.go:110 | DisambiguateProperties_buildRenamingIndex | 2 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_DisambiguateProperties.go:276 | DisambiguateProperties_PropertyReferenceIndexJson.init_DisambiguateProperties_PropertyReferenceIndexJson | 2 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_DisambiguateProperties.go:106 | DisambiguateProperties_buildRenamingIndex | 15 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_DisambiguateProperties.go:110 | DisambiguateProperties_buildRenamingIndex | 15 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_DisambiguateProperties.go:383 | DisambiguateProperties_TypeNodeJson.init_DisambiguateProperties_TypeNodeJson | 15 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_DisambiguateProperties.go:383 | DisambiguateProperties_TypeNodeJson.init_DisambiguateProperties_TypeNodeJson | 15 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_FeatureSet.go:146 | FeatureSet.withFeatureVarargs | 2 | T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place | gap for a rule | file unchanged |
| g_FindModuleDependencies.go:49 | FindModuleDependencies.process | 15 | T2.16a TreeSet.tailSet / Map.containsValue on a typed value | gap for a rule | file unchanged |
| g_FunctionType.go:977 | FunctionType.getConstructorOnlyTemplateParameters | 11 | T2.13 subList / reverse / asList of an immutable typed list | gap for a rule | file unchanged |
| g_IRFactory.go:638 | IRFactory.transform | 15 | T2.11 RangeSet<LineAndColumn> / Range.closed | gap for a rule | same line; pattern 2 -> 2 |
| g_IRFactory.go:638 | IRFactory.transform | 15 | T2.11 RangeSet<LineAndColumn> / Range.closed | gap for a rule | same line; pattern 2 -> 2 |
| g_JSDocInfo.go:41 | JSDocInfo.asMap | 0 | T2.7 TreeMap<Property<?>, Object> of JSDocInfo | counted reason | same line; pattern 1 -> 1 |
| g_JSDocInfo.go:2082 | JSDocInfo_Builder.finit_JSDocInfo_Builder | 0 | T2.7 TreeMap<Property<?>, Object> of JSDocInfo | counted reason | same line; pattern 1 -> 1 |
| g_JSDocInfo.go:605 | JSDocInfo.getSuppressionsAndTheirDescription | 2 | T2.15 ImmutableMap<ImmutableSet<String>, String> (a collection as a key) | counted reason | no such line in the translation |
| g_JSDocInfo.go:2378 | JSDocInfo_Builder.recordSuppressions_2 | 2 | T2.15 ImmutableMap<ImmutableSet<String>, String> (a collection as a key) | counted reason | same line; pattern 1 -> 1 |
| g_JSDocInfo.go:593 | JSDocInfo.getSuppressions | 10 | T2.15 ImmutableMap<ImmutableSet<String>, String> (a collection as a key) | counted reason | closest line; pattern 1 -> 1 |
| g_JSDocInfo.go:15 | JSDocInfo.init_JSDocInfo | 11 | T2.7 TreeMap<Property<?>, Object> of JSDocInfo | counted reason | same line; pattern 1 -> 1 |
| g_JSDocInfo.go:67 | JSDocInfo.toBuilder_1 | 11 | T2.7 TreeMap<Property<?>, Object> of JSDocInfo | counted reason | same line; pattern 1 -> 1 |
| g_JSDocInfo.go:590 | JSDocInfo.getSuppressions | 11 | T2.15 ImmutableMap<ImmutableSet<String>, String> (a collection as a key) | counted reason | closest line; pattern 1 -> 1 |
| g_JSDocInfo.go:2302 | JSDocInfo_Builder.recordTemplateTypeName_2 | 15 | T2.7 TreeMap<Property<?>, Object> of JSDocInfo | counted reason | closest line; pattern 1 -> 1 |
| g_JSDocInfo.go:2386 | JSDocInfo_Builder.recordSuppressions_2 | 15 | T2.15 ImmutableMap<ImmutableSet<String>, String> (a collection as a key) | counted reason | same line; pattern 1 -> 1 |
| g_JSDocInfo.go:2813 | JSDocInfo_Builder.setType | 15 | T2.7 TreeMap<Property<?>, Object> of JSDocInfo | counted reason | same line; pattern 1 -> 1 |
| g_JSDocInfo.go:2814 | JSDocInfo_Builder.setType | 15 | T2.7 TreeMap<Property<?>, Object> of JSDocInfo | counted reason | same line; pattern 1 -> 1 |
| g_JSDocInfo.go:2815 | JSDocInfo_Builder.setType | 15 | T2.7 TreeMap<Property<?>, Object> of JSDocInfo | counted reason | same line; pattern 1 -> 1 |
| g_JSDocInfoPrinter.go:219 | JSDocInfoPrinter.print | 10 | T2.15 ImmutableMap<ImmutableSet<String>, String> (a collection as a key) | counted reason | file unchanged |
| g_JSDocInfoPrinter.go:221 | JSDocInfoPrinter.print | 16 | T2.15 ImmutableMap<ImmutableSet<String>, String> (a collection as a key) | counted reason | file unchanged |
| g_JSType.go:606 | JSType.getTypeParameters | 11 | T2.13 subList / reverse / asList of an immutable typed list | gap for a rule | closest line; pattern 1 -> 1 |
| g_JSTypeReconserializer.go:323 | JSTypeReconserializer.getColorIdToJSTypeMapForDebugging | 16 | T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | gap for a rule | closest line; pattern 1 -> 1 |
| g_JSTypeRegistry.go:410 | JSTypeRegistry.initializeBuiltInTypes | 2 | T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place | gap for a rule | file unchanged |
| g_JSTypeRegistry.go:649 | JSTypeRegistry.resolveViaClosureNamespace | 13 | T2.13 subList / reverse / asList of an immutable typed list | gap for a rule | file unchanged |
| g_JSTypeRegistry.go:410 | JSTypeRegistry.initializeBuiltInTypes | 15 | T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place | gap for a rule | file unchanged |
| g_JsMessage.go:82 | JsMessage.getGenderedMessageVariants | 13 | T2.13 subList / reverse / asList of an immutable typed list | gap for a rule | file unchanged |
| g_Keywords.go:291 | Keywords_clinit | 0 | T2.8 EnumMap / IdentityHashMap / LinkedList made for a typed place | gap for a rule | file unchanged |
| g_LogsGson.go:36 | LogsGson_write | 2 | T2.17 an Object parameter or an Object value: no typed place | counted reason | file unchanged |
| g_ModuleLoader.go:74 | ModuleLoader.createRootPaths | 2 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_ModuleLoader.go:90 | ModuleLoader.createRootPaths | 13 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_ModuleLoader.go:85 | ModuleLoader.createRootPaths | 15 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_ModuleLoader.go:87 | ModuleLoader.createRootPaths | 15 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_ModuleLoader.go:79 | ModuleLoader.createRootPaths | 16 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_ModuleLoader.go:79 | ModuleLoader.createRootPaths | 16 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_NamedType.go:207 | NamedType.resolveInternal_impl | 11 | T2.13 subList / reverse / asList of an immutable typed list | gap for a rule | file unchanged |
| g_NodeModuleResolver.go:71 | NodeModuleResolver_buildNodeModulesFoldersRegistry | 2 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_PathUtil.go:46 | PathUtil_removeExtraneousSlashes | 15 | T2.12 CharMatcher / Splitter with a String argument | gap for a rule | file unchanged |
| g_PathUtil.go:50 | PathUtil_removeExtraneousSlashes | 15 | T2.12 CharMatcher / Splitter with a String argument | gap for a rule | file unchanged |
| g_PrebuildAst.go:28 | PrebuildAst.prebuild | 15 | T2.16b an Object value nobody reads typed / an Iterable<?> dispatched by instanceof | counted reason | file unchanged |
| g_ProcessDefines.go:207 | ProcessDefines.collectDefines | 2 | T2.4 builders whose chain Colls does not type | gap for a rule | file unchanged |
| g_PropertyMap.go:177 | PropertyMap.getAllKeys | 2 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_PureFunctionIdentifier.go:496 | PureFunctionIdentifier_FunctionBodyAnalyzer.finit_PureFunctionIdentifier_FunctionBodyAnalyzer | 2 | T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place | gap for a rule | file unchanged |
| g_RefactoringDriver.go:91 | RefactoringDriver_Builder.addExternsFromFile | 2 | T2.6 immutable copies into a typed place | gap for a rule | file unchanged |
| g_RefactoringDriver.go:117 | RefactoringDriver_Builder.addInputsFromFile | 2 | T2.6 immutable copies into a typed place | gap for a rule | file unchanged |
| g_ReplaceStrings.go:282 | ReplaceStrings.parseConfiguration | 2 | T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place | gap for a rule | file unchanged |
| g_RequireAliasGenerator.go:11 | RequireAliasGenerator.init_RequireAliasGenerator | 2 | T2.6 immutable copies into a typed place | gap for a rule | file unchanged |
| g_RequireAliasGenerator.go:11 | RequireAliasGenerator.init_RequireAliasGenerator | 13 | T2.6 immutable copies into a typed place | gap for a rule | file unchanged |
| g_Scope.go:187 | Scope.getOwnImplicitSlot | 0 | T2.8 EnumMap / IdentityHashMap / LinkedList made for a typed place | gap for a rule | file unchanged |
| g_ScopedAliases.go:449 | ScopedAliases_Traversal.finit_ScopedAliases_Traversal | 2 | T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place | gap for a rule | file unchanged |
| g_SerializeTypesToPointers.go:79 | SerializeTypesToPointers.logTypeMismatches | 15 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_Source.go:243 | Source_Builder.addRuntime | 2 | T2.4 builders whose chain Colls does not type | gap for a rule | file unchanged |
| g_SuggestedFix.go:44 | SuggestedFix.getNonDefaultAlternatives | 11 | T2.13 subList / reverse / asList of an immutable typed list | gap for a rule | file unchanged |
| g_SymbolTable.go:531 | SymbolTable.removeSymbol | 2 | T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | gap for a rule | closest line; pattern 1 -> 1 |
| g_SymbolTable.go:903 | SymbolTable.flattenGoogModuleExports | 2 | T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | gap for a rule | closest line; pattern 1 -> 1 |
| g_SymbolTable.go:924 | SymbolTable.mergeExternSymbolsDuplicatedOnWindow | 2 | T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | gap for a rule | closest line; pattern 1 -> 1 |
| g_SymbolTable.go:946 | SymbolTable.removeGeneratedSymbols | 2 | T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | gap for a rule | closest line; pattern 1 -> 1 |
| g_SymbolTable.go:532 | SymbolTable.removeSymbol | 11 | T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | gap for a rule | closest line; pattern 1 -> 1 |
| g_SymbolTable.go:904 | SymbolTable.flattenGoogModuleExports | 11 | T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | gap for a rule | closest line; pattern 1 -> 1 |
| g_SymbolTable.go:925 | SymbolTable.mergeExternSymbolsDuplicatedOnWindow | 11 | T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | gap for a rule | closest line; pattern 1 -> 1 |
| g_SymbolTable.go:947 | SymbolTable.removeGeneratedSymbols | 11 | T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) | gap for a rule | closest line; pattern 1 -> 1 |
| g_SymbolTable.go:1842 | SymbolTable_5.compareT_ | 15 | T2.9 Ordering<T> fields | gap for a rule | same line; pattern 2 -> 2 |
| g_SymbolTable.go:1842 | SymbolTable_5.compareT_ | 15 | T2.9 Ordering<T> fields | gap for a rule | same line; pattern 2 -> 2 |
| g_SymbolTable.go:1883 | SymbolTable_6.compareT_ | 15 | T2.9 Ordering<T> fields | gap for a rule | same line; pattern 2 -> 2 |
| g_SymbolTable.go:1883 | SymbolTable_6.compareT_ | 15 | T2.9 Ordering<T> fields | gap for a rule | same line; pattern 2 -> 2 |
| g_SymbolTable.go:1932 | SymbolTable_7.compareT_ | 15 | T2.9 Ordering<T> fields | gap for a rule | same line; pattern 2 -> 2 |
| g_SymbolTable.go:1932 | SymbolTable_7.compareT_ | 15 | T2.9 Ordering<T> fields | gap for a rule | same line; pattern 2 -> 2 |
| g_SymbolTable.go:1934 | SymbolTable_7.compareT_ | 15 | T2.9 Ordering<T> fields | gap for a rule | same line; pattern 2 -> 2 |
| g_SymbolTable.go:1934 | SymbolTable_7.compareT_ | 15 | T2.9 Ordering<T> fields | gap for a rule | same line; pattern 2 -> 2 |
| g_TemplateTypeReplacer.go:348 | TemplateTypeReplacer.getVisitedObjectTypeOrNull | 0 | T2.8 EnumMap / IdentityHashMap / LinkedList made for a typed place | gap for a rule | file unchanged |
| g_Timeline.go:69 | Timeline.addEvent | 11 | T2.14 Map<V, Event<V>> of a generic method (erased type variable) | counted reason | file unchanged |
| g_Tracer.go:249 | Tracer_getThreadTrace | 11 | T2.10 ThreadLocal<T> | gap for a rule | file unchanged |
| g_Tracer.go:253 | Tracer_getThreadTrace | 15 | T2.10 ThreadLocal<T> | gap for a rule | file unchanged |
| g_TypeStringBuilder.go:58 | TypeStringBuilder.appendAll | 16 | T2.16b an Object value nobody reads typed / an Iterable<?> dispatched by instanceof | counted reason | file unchanged |
| g_TypeTransformation.go:101 | TypeTransformation.addNewEntry | 2 | T2.18 a generic method's type variable | counted reason | file unchanged |
| g_TypeTransformation.go:101 | TypeTransformation.addNewEntry | 15 | T2.18 a generic method's type variable | counted reason | file unchanged |
| g_TypedScope.go:335 | TypedScope.getOwnImplicitSlot | 0 | T2.8 EnumMap / IdentityHashMap / LinkedList made for a typed place | gap for a rule | file unchanged |
| g_TypedScopeCreator.go:96 | TypedScopeCreator.getAllMemoizedScopes | 2 | T2.6 immutable copies into a typed place | gap for a rule | file unchanged |
| g_VariableMap.go:28 | VariableMap.getOriginalNameToNewNameMap | 2 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_VariableMap.go:53 | VariableMap.toBytes | 2 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_VariableMap.go:58 | VariableMap.toBytes | 10 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_VariableMap.go:58 | VariableMap.toBytes | 10 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_VariableMap.go:60 | VariableMap.toBytes | 10 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_VariableMap.go:61 | VariableMap.toBytes | 16 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_VariableMap.go:63 | VariableMap.toBytes | 16 | T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders | gap for a rule | file unchanged |
| g_Timeline.go:68 | Timeline.addEvent | 100 | T2.14 Map<V, Event<V>> of a generic method (erased type variable) | counted reason | file unchanged |

</details>

### T3: 28 sites

| category | sites | verdict | reason, or the rule that would take them | jsgo |
|---|---:|---|---|---|
| T3.4 Hasher.putUnencodedChars | 1 | gap for a rule 1 | `Conv.strSeq`: a String variant of the CharSequence parameter | `internal/jscomp/random_name_generator.go:308`: hand murmur3 over a typed slice |
| T3.1 contains / indexOf / remove(Object) on a List or Collection | 16 | gap for a rule 16 | piece objarg extended to lists: `contains` / `indexOf` / `remove(Object)` of a typed List with an argument of the element type call typed twins (a scan with the element's equals) instead of boxing the argument (objarg: "no typed lookup: a List or Collection receiver"). It re-takes the list twins that COLLECTIONS-PLAN 10.8 built and removed for binary size, so the size cost has to be measured again | `internal/jscomp/pass_list_builder.go:192`, `internal/jscomp/compiler_input.go:302`, `internal/jscomp/template_ast_matcher.go:233`: `slices.Contains` / `slices.Index` on typed slices |
| T3.2 Map.get / remove on a wrapped erased map | 2 | gap for a rule 2 | both maps are erased objects behind `mapAs_` (`new ConcurrentHashMap<>()`, a `LinkedHashMap`): make them typed maps (makers, as T2.8), then add the key/value pairs to `tools/j2go/objarg-pairs.txt` | `internal/jscomp/compiler_init.go:345`: a typed Go map |
| T3.3a List.add(int, E) on a typed list | 2 | gap for a rule 2 | a typed twin `add_int_t(i, x T)` on `ListT` (`TList.insert` exists in rt_tlist.go) | `internal/jscomp/compiler_sources.go:732`: `slices.Insert` |
| T3.3c a generic method's type variable | 1 | counted reason 1 | as T2.14: `<V> addEvent(V, Map<V, Event<V>>)`, V is Object in Go | `internal/jscomp/timeline.go:101` |
| T3.3b a map whose values are Object in Java | 6 | counted reason 6 | the value type is Object in Java: `LinkedHashMap<String, Object>` of define values (Boolean / Number / String) and of JSON fields, `Map<String, Object>` of source-map extensions | `internal/jscomp/compiler_options_preprocess.go:135` (hand tagged struct), `internal/sourcemap/source_map_object.go:23` (`Value any`) |

T3 total 28: gap for a rule 21, counted reason 7

<details><summary>sites</summary>

| site (eighth flip tree) | function | census row | category | verdict | check (base -> main) |
|---|---|---:|---|---|---|
| g_Compiler.go:355 | Compiler.getTypedAstDeserializer_impl | 15 | T3.2 Map.get / remove on a wrapped erased map | gap for a rule | file unchanged |
| g_Compiler.go:2129 | Compiler.getIndexUntyped_impl | 15 | T3.2 Map.get / remove on a wrapped erased map | gap for a rule | file unchanged |
| g_Compiler.go:2631 | Compiler.getSynthesizedExternsInput_impl | 15 | T3.3a List.add(int, E) on a typed list | gap for a rule | file unchanged |
| g_Compiler.go:2653 | Compiler.getSynthesizedTypeSummaryInput_impl | 15 | T3.3a List.add(int, E) on a typed list | gap for a rule | file unchanged |
| g_CompilerInput.go:191 | CompilerInput.addOrderedRequire | 15 | T3.1 contains / indexOf / remove(Object) on a List or Collection | gap for a rule | file unchanged |
| g_CompilerInput.go:390 | CompilerInput_DepsFinder.visitSubtree | 15 | T3.1 contains / indexOf / remove(Object) on a List or Collection | gap for a rule | file unchanged |
| g_CompilerOptions.go:854 | CompilerOptions.setDefineToStringLiteral | 15 | T3.3b a map whose values are Object in Java | counted reason | closest line; pattern 1 -> 1 |
| g_DisambiguateProperties.go:399 | DisambiguateProperties_TypeNodeJson.toLogsGson | 15 | T3.3b a map whose values are Object in Java | counted reason | file unchanged |
| g_Es6ConvertSuperConstructorCalls.go:552 | Es6ConvertSuperConstructorCalls_1.shouldTraverse | 15 | T3.1 contains / indexOf / remove(Object) on a List or Collection | gap for a rule | file unchanged |
| g_EsModuleProcessor.go:242 | EsModuleProcessor_FindMutableExports.visit_impl | 15 | T3.1 contains / indexOf / remove(Object) on a List or Collection | gap for a rule | file unchanged |
| g_FeatureCollector.go:195 | FeatureCollector.maybeAddTopLevelAwait | 15 | T3.1 contains / indexOf / remove(Object) on a List or Collection | gap for a rule | file unchanged |
| g_FindModuleDependencies.go:43 | FindModuleDependencies.process | 15 | T3.1 contains / indexOf / remove(Object) on a List or Collection | gap for a rule | file unchanged |
| g_Invalidation.go:50 | Invalidation.toLogsGson_impl | 15 | T3.3b a map whose values are Object in Java | counted reason | file unchanged |
| g_Invalidation.go:163 | Invalidation_WithReceiverType.toLogsGson_impl | 15 | T3.3b a map whose values are Object in Java | counted reason | file unchanged |
| g_J2clConstantHoisterPass.go:67 | J2clConstantHoisterPass.maybeHoistClassField | 15 | T3.1 contains / indexOf / remove(Object) on a List or Collection | gap for a rule | file unchanged |
| g_PassListBuilder.go:89 | PassListBuilder.contains | 15 | T3.1 contains / indexOf / remove(Object) on a List or Collection | gap for a rule | file unchanged |
| g_PassListBuilder.go:109 | PassListBuilder.assertPassOrder | 15 | T3.1 contains / indexOf / remove(Object) on a List or Collection | gap for a rule | file unchanged |
| g_PassListBuilder.go:110 | PassListBuilder.assertPassOrder | 15 | T3.1 contains / indexOf / remove(Object) on a List or Collection | gap for a rule | file unchanged |
| g_PolymerClassDefinition.go:257 | PolymerClassDefinition_overwriteMembersIfPresent | 15 | T3.1 contains / indexOf / remove(Object) on a List or Collection | gap for a rule | file unchanged |
| g_ProcessCommonJSModules.go:619 | ProcessCommonJSModules_FindImportsAndExports.initializeModule | 15 | T3.1 contains / indexOf / remove(Object) on a List or Collection | gap for a rule | file unchanged |
| g_ProcessCommonJSModules.go:657 | ProcessCommonJSModules_FindImportsAndExports.initializeModule | 15 | T3.1 contains / indexOf / remove(Object) on a List or Collection | gap for a rule | file unchanged |
| g_RandomNameGenerator.go:147 | RandomNameGenerator.generateSuffix | 15 | T3.4 Hasher.putUnencodedChars | gap for a rule | file unchanged |
| g_RenameVars.go:222 | RenameVars.getLocalVarIndex | 15 | T3.1 contains / indexOf / remove(Object) on a List or Collection | gap for a rule | file unchanged |
| g_SourceMapObjectParser.go:86 | SourceMapObjectParser_parse | 15 | T3.3b a map whose values are Object in Java | counted reason | file unchanged |
| g_SourceMapObjectParser.go:189 | SourceMapObjectParser_parseFastInto | 15 | T3.3b a map whose values are Object in Java | counted reason | file unchanged |
| g_TemplateAstMatcher.go:384 | TemplateAstMatcher_1.visit | 15 | T3.1 contains / indexOf / remove(Object) on a List or Collection | gap for a rule | file unchanged |
| g_TemplateAstMatcher.go:386 | TemplateAstMatcher_1.visit | 15 | T3.1 contains / indexOf / remove(Object) on a List or Collection | gap for a rule | file unchanged |
| g_Timeline.go:79 | Timeline.addEvent | 15 | T3.3c a generic method's type variable | counted reason | file unchanged |

</details>

### T4: 63 sites

| category | sites | verdict | reason, or the rule that would take them | jsgo |
|---|---:|---|---|---|
| T4.4 not a site: placeholder stubs of AbstractMap | 4 | counted reason 4 | not a site: placeholder methods of `AbstractMap` in g_zstubs.go (no Java class of the program names AbstractMap) | - |
| T4.1b compareTo of JSDocInfo.Property (Comparable<Property<Object>>) | 2 | counted reason 2 | not a bridge: the only, erased `compareTo(any)`; piece cmp refuses `Comparable` of another type than the class (`Property<?>` implements `Comparable<Property<Object>>`) | `internal/rhino/js_doc_info.go:392` |
| T4.1 Comparable / Comparator bridges kept for erased callers | 36 | gap for a rule 36 | type the erased callers that keep javac's bridge (COLLECTIONS-PLAN 11.2): Guava `ImmutableSortedSet.naturalOrder()` builders and `copyOf` (T2.2), `Arrays.sort(Object[])` (a twin `Arrays_sortT_`), `TreeRangeSet` (T2.11), Guava `Ordering` (T2.9); then a deadbridge-style check (no erased compare reaches it, 11.5) drops the bridge | `internal/jscomp/diagnostic_type.go:209`, `internal/jscomp/default_name_generator.go:327`, `internal/jscomp/js_chunk_graph.go:578`: typed `CompareTo(*T)` only, no bridge |
| T4.2a Iterator<T> bridges (next returns Object) | 4 | gap for a rule 4 | as T4.1: the bridge serves the erased `iterator()` path (`Iterator_un_(this.iterator_t())`) that erased runtime code reads; typing those readers lets a deadbridge-style check drop it | `internal/jscomp/node_iterators.go:129`: typed `Next() rhino.Node`, no bridge |
| T4.2b computeNext of Guava AbstractIterator<T> | 3 | gap for a rule 3 | not bridges: the runtime's `AbstractIterator` is erased; a typed `AbstractIteratorT[T]` (`computeNext_t`), as `next_t` | `internal/refactoring/require_alias_generator.go:116`, `internal/jscomp/node_util_functions_classes.go:936`: hand cursor / `iter.Seq` |
| T4.3a clone bridges | 2 | gap for a rule 2 | dead bridges: the program calls `clone_rX` directly, and the only erased caller (`Object_clone`) is given a BitSet at all three of its call sites; a deadbridge-style check drops them | `internal/rhino/js_doc_info.go:807`: typed `Clone()`, no bridge |
| T4.3b CacheLoader.load of CachingTranspiler | 3 | gap for a rule 3 | not a bridge: Guava's CacheLoader is erased in the runtime; with T1.11 (typed LoadingCache / CacheLoader) `load` takes the key at its type | `internal/transpile/caching_transpiler.go:55`: hand typed cache |
| T4.3c AbstractSet<String>.add of an anonymous class | 2 | gap for a rule 2 | not a javac bridge: the runtime's AbstractSet is erased; a typed `AbstractSetT[T]` (`add_t`) as for the collection interfaces | `internal/jscomp/compiler.go:349`: hand typed set |
| T4.3d Lazy<String> of Source (erased generic) | 3 | gap for a rule 3 | with T1.21: `Source.Lazy<T>` is erased (J2GO_GENERICS: "used raw" at Source.java:126-127, `Source.@Nullable Lazy<String>`, where the type-annotated qualified type loses its type arguments, a translator defect; and "a captured type" in its equals, Source.java:329, an ownwild-like equals not of the listed shape); fixing the first and listing the equals in ownwild-equals.txt types it | `internal/bundle/source.go:35`: `lazyString` |
| T4.3e a parameter Java declares Object | 3 | counted reason 3 | Java declares Object: `AbstractSet.contains(Object)`, `Predicate<Object>.apply(Object)`, `contains(Object)` | `internal/jscomp/compiler.go:349` (hand set); StandardUnionFind not ported generically |
| T4.3f Function<Integer, Void> | 1 | counted reason 1 | `Function<Integer, Void>`: a boxed Integer argument and a Void result held as `any` (rule "function types" makes boxed type arguments `any`; Void has no value); jsgo dropped the Void result by hand | `internal/cli/command_line_config_specs.go:342`: `Apply(exitCode int)` |

T4 total 63: gap for a rule 53, counted reason 10

<details><summary>sites</summary>

| site (eighth flip tree) | function | census row | category | verdict | check (base -> main) |
|---|---|---:|---|---|---|
| g_CachingTranspiler.go:101 | CachingTranspiler_1.load | 11 | T4.3b CacheLoader.load of CachingTranspiler | gap for a rule | file unchanged |
| g_CachingTranspiler.go:103 | CachingTranspiler_1.load | 15 | T4.3b CacheLoader.load of CachingTranspiler | gap for a rule | file unchanged |
| g_CheckRequiresSorted.go:407 | CheckRequiresSorted_DestructuringBinding.compareTo | 11 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_CheckRequiresSorted.go:582 | CheckRequiresSorted_ImportStatement.compareTo | 11 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_CodeReplacement.go:53 | CodeReplacement.compareTo | 11 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_Compiler.go:3695 | Compiler_1.add | 16 | T4.3c AbstractSet<String>.add of an anonymous class | gap for a rule | file unchanged |
| g_DefaultNameGenerator.go:238 | DefaultNameGenerator_CharPriority.compareTo | 11 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_DefaultNameGenerator.go:242 | DefaultNameGenerator_CharPriority.clone | 15 | T4.3a clone bridges | gap for a rule | file unchanged |
| g_DiagnosticType.go:79 | DiagnosticType.compareTo | 11 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_DisambiguateProperties.go:300 | DisambiguateProperties_PropertyReferenceIndexJson.compareTo | 11 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_DisambiguateProperties.go:354 | DisambiguateProperties_PropertyReferenceJson.compareTo | 11 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_DisambiguateProperties.go:429 | DisambiguateProperties_TypeEdgeJson.compareTo | 11 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_IRFactory.go:1485 | IRFactory_LineAndColumn.compareTo | 11 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | same line; pattern 1 -> 1 |
| g_JSChunkGraph.go:613 | JSChunkGraph_InverseDepthComparator.compare | 11 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_JSChunkGraph.go:613 | JSChunkGraph_InverseDepthComparator.compare | 11 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_JSDocInfo.go:984 | JSDocInfo_Property.compareTo | 11 | T4.1b compareTo of JSDocInfo.Property (Comparable<Property<Object>>) | counted reason | same line; pattern 1 -> 1 |
| g_JSDocInfo.go:808 | JSDocInfo.clone | 15 | T4.3a clone bridges | gap for a rule | same line; pattern 1 -> 1 |
| g_Node.go:3492 | Node_SiblingNodeIterator.next | 15 | T4.2a Iterator<T> bridges (next returns Object) | gap for a rule | file unchanged |
| g_Node.go:3926 | Node_AncestorIterable_1.next | 15 | T4.2a Iterator<T> bridges (next returns Object) | gap for a rule | file unchanged |
| g_NodeIterators.go:74 | NodeIterators_FunctionlessLocalScope.next | 15 | T4.2a Iterator<T> bridges (next returns Object) | gap for a rule | file unchanged |
| g_NodeIterators.go:165 | NodeIterators_LocalVarMotion.next | 15 | T4.2a Iterator<T> bridges (next returns Object) | gap for a rule | file unchanged |
| g_NodeUtil.go:4751 | NodeUtil_PreOrderIterator.computeNext | 15 | T4.2b computeNext of Guava AbstractIterator<T> | gap for a rule | file unchanged |
| g_NodeUtil.go:4986 | NodeUtil_TemplateArgsIterable_1.computeNext | 15 | T4.2b computeNext of Guava AbstractIterator<T> | gap for a rule | file unchanged |
| g_RequireAliasGenerator.go:115 | RequireAliasGenerator_NameIterator.computeNext | 15 | T4.2b computeNext of Guava AbstractIterator<T> | gap for a rule | file unchanged |
| g_SerializeTypesToPointers.go:255 | SerializeTypesToPointers_TypeMismatchJson.compareTo | 11 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_Source.go:434 | Source_Lazy_ofInstance | 15 | T4.3d Lazy<String> of Source (erased generic) | gap for a rule | file unchanged |
| g_Source.go:57 | Source.code | 16 | T4.3d Lazy<String> of Source (erased generic) | gap for a rule | file unchanged |
| g_Source.go:61 | Source.originalCode | 16 | T4.3d Lazy<String> of Source (erased generic) | gap for a rule | file unchanged |
| g_SymbolTable.go:1892 | SymbolTable_6.compare | 11 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | closest line; pattern 1 -> 1 |
| g_SymbolTable.go:1892 | SymbolTable_6.compare | 11 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | closest line; pattern 1 -> 1 |
| g_SymbolTable.go:1939 | SymbolTable_7.compare | 11 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | closest line; pattern 1 -> 1 |
| g_SymbolTable.go:1939 | SymbolTable_7.compare | 11 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | closest line; pattern 1 -> 1 |
| g_SymbolTable.go:1851 | SymbolTable_5.compare | 16 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | no such line in the translation |
| g_SymbolTable.go:1851 | SymbolTable_5.compare | 16 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | no such line in the translation |
| g_AbstractCommandLineRunner.go:3049 | AbstractCommandLineRunner_SystemExitCodeReceiver.apply | 100 | T4.3f Function<Integer, Void> | counted reason | same line; pattern 2 -> 2 |
| g_CachingTranspiler.go:100 | CachingTranspiler_1.load | 100 | T4.3b CacheLoader.load of CachingTranspiler | gap for a rule | file unchanged |
| g_CheckRequiresSorted.go:406 | CheckRequiresSorted_DestructuringBinding.compareTo | 100 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_CheckRequiresSorted.go:581 | CheckRequiresSorted_ImportStatement.compareTo | 100 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_CodeReplacement.go:52 | CodeReplacement.compareTo | 100 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_Compiler.go:3690 | Compiler_1.contains | 100 | T4.3e a parameter Java declares Object | counted reason | file unchanged |
| g_Compiler.go:3694 | Compiler_1.add | 100 | T4.3c AbstractSet<String>.add of an anonymous class | gap for a rule | file unchanged |
| g_DefaultNameGenerator.go:237 | DefaultNameGenerator_CharPriority.compareTo | 100 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_DiagnosticType.go:78 | DiagnosticType.compareTo | 100 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_DisambiguateProperties.go:299 | DisambiguateProperties_PropertyReferenceIndexJson.compareTo | 100 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_DisambiguateProperties.go:353 | DisambiguateProperties_PropertyReferenceJson.compareTo | 100 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_DisambiguateProperties.go:428 | DisambiguateProperties_TypeEdgeJson.compareTo | 100 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_IRFactory.go:1484 | IRFactory_LineAndColumn.compareTo | 100 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | same line; pattern 1 -> 1 |
| g_JSChunkGraph.go:612 | JSChunkGraph_InverseDepthComparator.compare | 100 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_JSChunkGraph.go:612 | JSChunkGraph_InverseDepthComparator.compare | 100 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_JSDocInfo.go:983 | JSDocInfo_Property.compareTo | 100 | T4.1b compareTo of JSDocInfo.Property (Comparable<Property<Object>>) | counted reason | same line; pattern 1 -> 1 |
| g_SerializeTypesToPointers.go:254 | SerializeTypesToPointers_TypeMismatchJson.compareTo | 100 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | file unchanged |
| g_StandardUnionFind.go:195 | StandardUnionFind_1.apply | 100 | T4.3e a parameter Java declares Object | counted reason | file unchanged |
| g_StandardUnionFind.go:237 | StandardUnionFind_2.contains | 100 | T4.3e a parameter Java declares Object | counted reason | file unchanged |
| g_SymbolTable.go:1850 | SymbolTable_5.compare | 100 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | same line; pattern 2 -> 2 |
| g_SymbolTable.go:1850 | SymbolTable_5.compare | 100 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | same line; pattern 2 -> 2 |
| g_SymbolTable.go:1891 | SymbolTable_6.compare | 100 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | same line; pattern 2 -> 2 |
| g_SymbolTable.go:1891 | SymbolTable_6.compare | 100 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | same line; pattern 2 -> 2 |
| g_SymbolTable.go:1938 | SymbolTable_7.compare | 100 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | same line; pattern 2 -> 2 |
| g_SymbolTable.go:1938 | SymbolTable_7.compare | 100 | T4.1 Comparable / Comparator bridges kept for erased callers | gap for a rule | same line; pattern 2 -> 2 |
| g_zstubs.go:94 | AbstractMap.containsKey | 100 | T4.4 not a site: placeholder stubs of AbstractMap | counted reason | file unchanged |
| g_zstubs.go:98 | AbstractMap.containsValue | 100 | T4.4 not a site: placeholder stubs of AbstractMap | counted reason | file unchanged |
| g_zstubs.go:118 | AbstractMap.put | 100 | T4.4 not a site: placeholder stubs of AbstractMap | counted reason | file unchanged |
| g_zstubs.go:118 | AbstractMap.put | 100 | T4.4 not a site: placeholder stubs of AbstractMap | counted reason | file unchanged |

</details>

### R4: 41 sites

| category | sites | verdict | reason, or the rule that would take them | jsgo |
|---|---:|---|---|---|
| R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | 41 | taken on main 41 | typed by piece ownwild on main (`SourcePosition_rhino[T]`, `AbstractCommandLineRunner_FlagEntry[T]`): see the check | N1 |

R4 total 41: taken on main 41

<details><summary>sites</summary>

| site (eighth flip tree) | function | census row | category | verdict | check (base -> main) |
|---|---|---:|---|---|---|
| g_AbstractCommandLineRunner.go:144 | AbstractCommandLineRunner_setWarningGuardOptions | 11 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | closest line; pattern 1 -> 0 |
| g_AbstractCommandLineRunner.go:148 | AbstractCommandLineRunner_setWarningGuardOptions | 11 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | closest line; pattern 1 -> 0 |
| g_AbstractCommandLineRunner.go:438 | AbstractCommandLineRunner.createInputs_4 | 11 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | closest line; pattern 1 -> 0 |
| g_AbstractCommandLineRunner.go:466 | AbstractCommandLineRunner.createInputs_4 | 11 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | closest line; pattern 1 -> 0 |
| g_AbstractCommandLineRunner.go:517 | AbstractCommandLineRunner.deduplicateIjsFiles | 11 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | closest line; pattern 2 -> 0 |
| g_AbstractCommandLineRunner.go:517 | AbstractCommandLineRunner.deduplicateIjsFiles | 11 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | closest line; pattern 2 -> 0 |
| g_AbstractCommandLineRunner.go:526 | AbstractCommandLineRunner.deduplicateIjsFiles | 11 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | closest line; pattern 1 -> 0 |
| g_AbstractCommandLineRunner.go:569 | AbstractCommandLineRunner.createSourceInputs | 15 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | closest line; pattern 1 -> 0 |
| g_AbstractCommandLineRunner.go:596 | AbstractCommandLineRunner.createExternInputs | 15 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:999 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 15 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | closest line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1002 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 15 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | closest line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1120 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 15 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | closest line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1123 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 15 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | closest line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1126 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 15 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | closest line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1585 | CommandLineRunner_Flags.getMixedJsSources | 15 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1594 | CommandLineRunner_Flags.getMixedJsSources | 15 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1604 | CommandLineRunner_Flags.getMixedJsSources | 15 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 1 -> 0 |
| g_JSDocInfo.go:1925 | JSDocInfo_TrimmedStringPosition.setItem_impl | 15 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 1 -> 0 |
| g_JSDocInfo.go:2201 | JSDocInfo_Builder.markAnnotation | 15 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 1 -> 0 |
| g_JSDocInfo.go:2226 | JSDocInfo_Builder.markText | 15 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 1 -> 0 |
| g_JSDocInfo.go:2235 | JSDocInfo_Builder.markTypeNode | 15 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 1 -> 0 |
| g_JSDocInfo.go:2245 | JSDocInfo_Builder.markName | 15 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 1 -> 0 |
| g_JSDocInfo.go:2254 | JSDocInfo_Builder.markName | 15 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 1 -> 0 |
| g_JSDocInfo.go:1920 | JSDocInfo_TrimmedStringPosition.setItem_impl | 16 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 1 -> 0 (line gone) |
| g_JSDocInfo.go:1959 | JSDocInfo_NamePosition.isEquivalentTo_impl | 16 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | closest line; pattern 2 -> 0 |
| g_JSDocInfo.go:1959 | JSDocInfo_NamePosition.isEquivalentTo_impl | 16 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | closest line; pattern 2 -> 0 |
| g_JSDocInfo.go:1962 | JSDocInfo_NamePosition.isEquivalentTo_impl | 16 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 3 -> 0 |
| g_JSDocInfo.go:1962 | JSDocInfo_NamePosition.isEquivalentTo_impl | 16 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 3 -> 0 |
| g_JSDocInfo.go:1962 | JSDocInfo_NamePosition.isEquivalentTo_impl | 16 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 3 -> 0 |
| g_JSDocInfo.go:2010 | JSDocInfo_TypePosition.isEquivalentTo_impl | 16 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | by hand: main g_JSDocInfo.go:1793 isEquivalentTo_impl(that *SourcePosition_rhino[Node]): getItem() is a Node |
| g_JSDocInfo.go:2010 | JSDocInfo_TypePosition.isEquivalentTo_impl | 16 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | by hand: main g_JSDocInfo.go:1793 isEquivalentTo_impl(that *SourcePosition_rhino[Node]): getItem() is a Node |
| g_JSDocInfo.go:2013 | JSDocInfo_TypePosition.isEquivalentTo_impl | 16 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 3 -> 0 |
| g_JSDocInfo.go:2013 | JSDocInfo_TypePosition.isEquivalentTo_impl | 16 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 3 -> 0 |
| g_JSDocInfo.go:2013 | JSDocInfo_TypePosition.isEquivalentTo_impl | 16 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 3 -> 0 |
| g_SymbolTable.go:704 | SymbolTable.fillJSDocInfo | 16 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 1 -> 0 |
| g_SymbolTable.go:711 | SymbolTable.fillJSDocInfo | 16 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 1 -> 0 |
| g_JSDocInfo.go:1919 | JSDocInfo_TrimmedStringPosition.setItem_impl | 100 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 1 -> 0 |
| g_JSDocInfo.go:2998 | JSDocInfo_TrimmedStringPosition.setItem | 100 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | closest line; pattern 1 -> 0 |
| g_SourcePosition_rhino.go:7 |  | 100 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | closest line; pattern 1 -> 0 |
| g_SourcePosition_rhino.go:41 | SourcePosition_rhino.setItem_impl | 100 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 1 -> 0 |
| g_SourcePosition_rhino.go:81 | SourcePosition_rhino.setItem | 100 | R4.1 SourcePosition<T> / FlagEntry<T> erased type parameter | taken on main | same line; pattern 1 -> 0 |

</details>

### R5: 56 sites

| category | sites | verdict | reason, or the rule that would take them | jsgo |
|---|---:|---|---|---|
| R5.2 not a site: `any` inside a string | 1 | counted reason 1 | not a site: the census's "declared any" pattern matched the words "if any," of the `--formatting` usage string | - |
| R5.3 Source.Lazy's captured instance | 2 | gap for a rule 2 | as T4.3d: `Source.Lazy<T>` erased; typing it makes the captured instance a T | `internal/bundle/source.go:35`: `lazyString` |
| R5.4 a boxed Long value of a typed map (computeIfAbsent's function) | 1 | gap for a rule 1 | as T1.16: piece nbox extended to type arguments (`Map<InstrumentationPoint, Long>` holds `Nbox[int64]`, the merge function takes and gives one); main has 6 such maps (4 Integer, 2 Long values) | not ported |
| R5.1 nullable boxed flags and their setters | 52 | taken on main 52 | typed by piece nbox on main (`Nbox[T]` fields, parameters and option setters): see the check | `internal/rhino/jstype_js_type.go:253` (a hand-packed flag) |

R5 total 56: gap for a rule 3, counted reason 1, taken on main 52

<details><summary>sites</summary>

| site (eighth flip tree) | function | census row | category | verdict | check (base -> main) |
|---|---|---:|---|---|---|
| g_CommandLineRunner.go:837 | CommandLineRunner_Flags.finit_CommandLineRunner_Flags | 15 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:870 | CommandLineRunner_Flags.finit_CommandLineRunner_Flags | 15 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_JSType.go:591 | JSType.hasAnyTemplateTypes | 15 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_OptimizeCalls.go:137 | OptimizeCalls_Builder.setConsiderExterns | 15 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_SubtypeChecker.go:61 | SubtypeChecker.setUsingStructuralSubtyping | 15 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_AbstractCommandLineRunner.go:2541 | AbstractCommandLineRunner_CommandLineConfig.setBrowserFeaturesetYear | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:702 |  | 100 | R5.1 nullable boxed flags and their setters | taken on main | closest line; pattern 1 -> 0 (line gone) |
| g_CommandLineRunner.go:721 |  | 100 | R5.1 nullable boxed flags and their setters | taken on main | closest line; pattern 1 -> 0 (line gone) |
| g_CommandLineRunner.go:733 |  | 100 | R5.1 nullable boxed flags and their setters | taken on main | closest line; pattern 1 -> 0 (line gone) |
| g_CommandLineRunner.go:950 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:955 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:960 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:965 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:970 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:975 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:980 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1041 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1046 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1056 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1061 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1111 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1116 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1145 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1150 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1160 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1165 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1170 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1180 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1190 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1195 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1198 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.2 not a site: `any` inside a string | counted reason | closest line; pattern 1 -> 1 |
| g_CommandLineRunner.go:1205 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1215 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1220 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1225 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1230 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1240 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1265 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1330 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1335 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1355 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1360 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1365 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1390 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1395 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1415 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1425 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1430 | CommandLineRunner_Flags.init_CommandLineRunner_Flags | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_CommandLineRunner.go:1650 | CommandLineRunner_Flags_addMultiFlagValue | 100 | R5.1 nullable boxed flags and their setters | taken on main | closest line; pattern 1 -> 0 |
| g_CompilerOptions.go:1106 | CompilerOptions.setPolymerVersion | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_JSType.go:9 |  | 100 | R5.1 nullable boxed flags and their setters | taken on main | by hand: main g_JSType.go:10 templateCheckResult Nbox[bool] |
| g_OptimizeCalls.go:114 |  | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 |
| g_ReportDecoder.go:137 | ReportDecoder_mergeProfiles | 100 | R5.4 a boxed Long value of a typed map (computeIfAbsent's function) | gap for a rule | file unchanged |
| g_Source.go:448 |  | 100 | R5.3 Source.Lazy's captured instance | gap for a rule | file unchanged |
| g_Source.go:462 | new_Source_Lazy_1 | 100 | R5.3 Source.Lazy's captured instance | gap for a rule | file unchanged |
| g_SubtypeChecker.go:9 |  | 100 | R5.1 nullable boxed flags and their setters | taken on main | same line; pattern 1 -> 0 (line gone) |

</details>

### Totals

| rule | sites | gap for a rule | counted reason | taken on main |
|---|---:|---:|---:|---:|
| T1 | 83 | 74 | 7 | 2 |
| T2 | 136 | 105 | 31 | 0 |
| T3 | 28 | 21 | 7 | 0 |
| T4 | 63 | 53 | 10 | 0 |
| R4 | 41 | 0 | 0 | 41 |
| R5 | 56 | 3 | 1 | 52 |
| all | 407 | 256 | 56 | 95 |

Gaps by category: T4.1 Comparable / Comparator bridges kept for erased callers 36, T2.2 Guava sorted immutables: copyOf / copyOfSorted / of / builders 27, T2.1 for-each over a fresh immutable copy or ImmutableList.of(..) 19, T2.3 ImmutableX.of(..) / copyOf(array) varargs for a typed place 18, T3.1 contains / indexOf / remove(Object) on a List or Collection 16, T1.2 ComparisonChain.compare(Comparable, Comparable) 12, T1.1 BigInteger.compareTo 10, T2.9 Ordering<T> fields 8, T2.4 builders whose chain Colls does not type 7, T1.6 Comparator.comparing / thenComparing with a key typed any 6, T2.13 subList / reverse / asList of an immutable typed list 6, T1.4 Object.clone() of a BitSet 6, T1.7 Lists.transform (a live view) 6, T1.14 converting closures of typed comparators given to Guava sorted makers 6, T2.8 EnumMap / IdentityHashMap / LinkedList made for a typed place 5, T1.5 Arrays.setAll / toArray of a typed array 5, T2.6 immutable copies into a typed place 5, T4.2a Iterator<T> bridges (next returns Object) 4, T1.11 CacheLoader.from / CacheBuilder<Object, Object> 3, T4.3b CacheLoader.load of CachingTranspiler 3, T1.12 a String given to a CharSequence parameter of a runtime function 3, T1.13 Map.putIfAbsent / Map.replace / Collections.singletonList on typed values 3, T4.2b computeNext of Guava AbstractIterator<T> 3, T1.16 function results that are a boxed Integer 3, T4.3d Lazy<String> of Source (erased generic) 3, T1.3 Enum.compareTo(E) 2, T2.16a TreeSet.tailSet / Map.containsValue on a typed value 2, T3.2 Map.get / remove on a wrapped erased map 2, T3.3a List.add(int, E) on a typed list 2, T4.3c AbstractSet<String>.add of an anonymous class 2, T2.19 typed comparator wrapped for ImmutableList.sortedCopyOf 2, T4.3a clone bridges 2, T1.8 for-each over Lists.reverse 2, T1.9 for-each over Splitter.split 2, T2.11 RangeSet<LineAndColumn> / Range.closed 2, T1.15 Iterables.concat of an Iterables.transform 2, T2.12 CharMatcher / Splitter with a String argument 2, T1.10 for-each over File.listFiles (Io_listFiles) 2, T2.10 ThreadLocal<T> 2, R5.3 Source.Lazy's captured instance 2, T3.4 Hasher.putUnencodedChars 1, T1.21 Supplier<String> given to Source.Builder (converting closure) 1, R5.4 a boxed Long value of a typed map (computeIfAbsent's function) 1
