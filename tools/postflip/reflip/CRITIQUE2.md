# CRITIQUE2: the "what stays" table tested against jsgo (independent review, 2026-11-19)

Tree: jm-reflip, branch reflip-trial at c7e2f715 (go/closure). jsgo: /home/user/jsgo at fa371e9
(non-test files under internal/). Nothing was built, gated or timed. Commands run: `census.py`,
`reflip/jsgocensus.py`, `go run ./castcensus -v`, `go run ./colls -kind K -n -v` (list, map, set,
treeset, treemap, immlist, immmap, immset, immmapb), `go run ./ranges -loops -n`,
`go run ./iters -n`, plus grep/python. (`go run` compiles only the report tools.) jsgo counterparts
were found through jsgo's `// Java: Class.method(...)` comments (30,542 of them).

Classes: (a) jsgo keeps an equivalent Java-ism; (b) jsgo removed it and a mechanical rule or tool
could too; (c) jsgo removed it only by changing behaviour or by a hand redesign; (d) the census
counts it but it is not a Java-ism.

## 1. Census corrections (these change the gaps before any row is argued)

1. **jsgo's cast target is 648, not 726.** jsgocensus.py's assertion regex
   `\.\(\*?[A-Za-z_][\w.]*...\)` also matches `.(type)`: its 648 already includes the 78 type
   switches (570 assertions + 78 switches). "648 + 78 = 726" counts the switches twice.
2. **Our 850 is not like-for-like with jsgo's 648.** jsgo's figure counts every `.(T)`, including
   its comma-ok instanceof tests. The same regex on our g_*.go gives 2,182. Of those, 1,563 are
   `this.self_.(T)` dispatch (the class model) and 98 are the helper bodies in g_0casts.go. That
   leaves 121 comma-ok tests, 219 other direct assertions (ParseTree's `asX()`, g_ParseTree.go:206
   etc., which is Java's own downcast) and 21 switches. Add the 71 `is_` calls and the comparable
   figure is about 850 + 71 + 121 + 219 + 21 ≈ 1,280 against 648.
3. **The any row is 934, but 337 of those are (d) by the table's own account:** 83 Go type
   parameters, 76 `return any(x)` lines, and 178 cast-helper operands (`icast_X(v any)`). The
   helper operands repeat the cast row: one helper per target type, and jsgo writes those casts
   inline. The comparable figures are about 597 against jsgo's 267 (281 minus its 11 + 3 of the
   same kinds).
4. **The "lambda objects" row (45) misses the typed adapters.** census.py matches `_fn\{func`.
   The 587 literals `&X_tfn_Y{...}` in g_*.go (541 wrapping a func literal, 46 a method value)
   are not counted anywhere. jsgo has 0 of them. 175 of the 587 are one consumer:
   `PassFactory_Builder.setInternalFactory` (g_DefaultPassConfig.go:155...).
5. **jsgo's Optional (57) misses jsgo's own Optional-of-string.** jsgo has `base.OptString`
   (internal/base/opt_string.go:5): 213 type spellings, 217 `SomeString(`, 112 `NoString()`, in 52
   files. It also has 243 functions returning `(string, bool)`. A fair jsgo figure is about 550
   Optional-like value sites plus 243 comma-ok results, not 57.
6. **castcensus's attribution is approximate.** For example, the 4 casts it books under
   "Compiler.up_AbstractCompiler" are `cast_[*CompilerInput](inputs.get(i))`
   (g_Compiler.go:1943, 2058, 2491; g_NestedCompilerRunner.go:117), which are erased List.get
   results.

## 2. Per row: samples, jsgo, classification

### Casts (850 calls; castcensus -v)

| # | our site | jsgo at the same method | class |
|---|---|---|---|
| 1 | g_FunctionType.go:955 `cast_[*FunctionType_Parameter](parameters.next())`, cursor `immCursorOf_(this.getParameters().asImm())` over a typed `*ImmutableList[*FunctionType_Parameter]` | rhino/jstype_function_type.go:941, index over `[]FunctionType_Parameter` | b: typed cursor `immCursorT_[T]` of a TList |
| 2 | g_Compiler.go:3047 `cast_[[]int8](this.runInCompilerThread(...))` | jscomp/compiler_pipeline.go:265 `Compiler_RunInCompilerThread[T any](c, func() T) T` | b: Java generic method `<T> T m(Callable<T>)` → package generic function |
| 3 | g_Compiler.go:3055 `cast_[[]int8](Iterables_getOnlyElement(Iterable(typedAstList)))` | compiler_state.go:304 (no list) | b: typed Iterables_getOnlyElementT_ |
| 4 | g_CommandLineRunner.go:571 `cast_[*ConformanceConfig](builder.build())` | cli/command_line_runner.go:904 `builder.Build()` typed | b: typed `buildT_()` per proto message |
| 5 | g_ConvertChunksToESModules.go:50 `cast_[*CompilerInput](el)` over `immElems_(chunk.getInputs())` | jscomp/js_chunk.go:300 `GetInputs() []*CompilerInput` | b: getInputs' typed result is blocked only by the method value `&Function_base_tfn_JSChunk_ImmutableList{(*JSChunk).getInputs}` (g_JSChunkGraph.go:159), which a typed Iterables_transform/concat removes (jsgo js_chunk_graph.go:267) |
| 6 | g_ColorGraphBuilder.go:63 `cast_[*ColorGraphNode](el2)` | disambiguate_color_graph_builder.go:131, `graph.LinkedDiGraphNode[*ColorGraphNode, EdgeReason]` | b: graph generics |
| 7 | g_BranchCoverageInstrumentationCallback.go:67 `cast_[ControlFlowGraph_Branch](outEdge.getValue())` | graph/graph.go:188 `DiGraph[N, E comparable]` | b: j2go dropped E from Graph/DiGraph (g_DiGraph.go:5 `DiGraph[N any]`; Java DiGraph.java:29 `DiGraph<N, E>`). Only 2 wildcard uses exist (DominatorTree.java:65, 166) |
| 8 | g_FunctionInjector.go:149 `icast_Map_Entry(el)` over `paramToArgMap.asImm().entrySet()`, with paramToArgMap a typed `*ImmutableMap[string, *ParamArgPair]` (line 146) | function_injector.go:249, typed | b: typed entry view of a typed map |
| 9 | g_DiagnosticToSuppressionMapper.go:36, same pattern | diagnostic_to_suppression_mapper.go:104 `range m.diagnosticGroups.All()` | b |
| 10 | g_RhinoErrorReporter.go:54, `RhinoErrorReporter_typeMap` made by a typed `immutableMapBuilder_(tc_p_Pattern, tc_p_DiagnosticType)...buildOrThrowErased()` (line 135) | rhino_error_reporter.go:227, typed | b: the builder is typed and only its build is erased |
| 11 | g_CheckRequiresSorted.go:580 bridge `compareToT_(cast_[...](other_))` | lint_check_requires_sorted.go:450 `slices.SortStableFunc` with a func | b: typed natural-order sort, then the bridge is dead |
| 12 | g_SubtypeChecker.go:146 `cast_[*EnumType](subtype.self())` | rhino/jstype_subtype_checker.go:232 `subtype.(*EnumType)` | a |
| 13 | g_TypedScopeCreator.go:1337 `to_FunctionType(ownerType.self())` | typed_scope_creator_types.go:371 `ownerType.(rhino.FunctionType)` | a |
| 14 | g_FunctionTypeBuilder.go:246 `to_ObjectType(maybeInterType.self())` | function_type_builder.go:419 `maybeInterType.(rhino.ObjectType)` | a |
| 15 | g_Es6ConvertSuperConstructorCalls.go:223 `cast_[*GlobalNamespace_Name](getSlot(..))` | global_namespace.go:608 `GetSlot` returns `rhino.StaticSlot` | a (covret could still split Java's covariant `Name getSlot`) |
| 16 | g_ConformanceRules.go:1040 `icast_CheckConformance_Precondition(this.requirementPrecondition.self())` | conformance_rules_banned.go:366 `return r.requirementPrecondition` | b: Java has no cast here (an enum upcast to an interface it implements); an `iup_` for value enums (nil for 0) does it |
| 17 | g_Parser.go:1263 `cast_[*TemplateLiteralToken](token.self())` | parser_literals_and_classes.go:69 `templateToken := token` (Token is one struct) | c: Token hierarchy redesigned |
| 18 | g_TypeInferencePass.go:92 cast of `Annotatable_getAnnotation(node)` | type_inference_pass.go:192, index arrays instead of annotations | c |
| 19 | g_ConvertChunksToESModules.go:262 `icast_Set(Map_computeIfAbsent(..., &Function_tfn_JSChunk_Set{...}))` | convert_chunks_to_es_modules.go:514, its own `ensureImports().ensure()` structs | c (b once the inner map is typed) |
| 20 | g_AstFactory.go:1157 `Iterables_getOnlyElement(iup_Iterable(x.getParameters().asImm())).(*FunctionType_Parameter)` | none (jsgo indexes) | b |

The sample comes out 13 b, 5 a, 2 c, but I chose it across castcensus's categories. Estimate from
those categories:

- **(b) ≈ 300 to 340:**
  - typed views of typed containers: 27 loops over `.asImm().entrySet/keySet/values` (64 casts,
    39 unboxings) and 22 other `.asImm().get/keySet` sites (10 casts);
  - typed cursors and immElems over `x.asImm()` of a TList: 14 cursors (~15 casts) and 21 loops
    with an element cast;
  - adapter bridges of runtime-only adapters: ~45 of the 98 `p0` casts;
  - proto `build()`: ~25;
  - generic methods and typed Iterables helpers: ~25;
  - graph E: ~20;
  - implsplit bridge callers: ~30;
  - foreach_ cursors and `el` of erased collections that colls types once item 7 below exists: ~60.
- **(a) ≈ 280:**
  - real downcasts of self(): ~65 of 80 (see castcensus);
  - the 13 Throwable.error() catches;
  - equals(Object) operands;
  - the JSDocInfo bag (jsgo has 90 `jsdocGet[T]` reads, rhino/js_doc_info.go:80);
  - the getSlot family (jsgo keeps StaticSlot).
- **(c) ≈ 200:**
  - Annotatable.getAnnotation 14;
  - Node property reads: getProp 8, getFeatureSet 6;
  - PassFactory and CmdLineParser stored functions;
  - passes jsgo restructured (TypeInferencePass, ConvertChunksToESModules, Token).
- **(d) ≈ 20:** castcensus misattributions and upcasts that castcensus books as casts.

### Fields/params declared any (934)

| # | our site | jsgo | class |
|---|---|---|---|
| 1 | g_0casts.go:5 `icast_X(v any)` (178 such) | inline `v.(T)` | d (repeats the cast row) |
| 2 | `[K any, V any]` (83) and `return any(x)` (76) | same regex: 11 + 3 | d |
| 3 | g_0lamparams.go:57 `Function_tfn_any_CompilerPass_cAbstractCompiler.apply(p0 any)` (175 literals) | pass_factory.go:69 `func(AbstractCompiler) CompilerPass`; Equals at :156/168 uses `that == f` plus `jutil.ObjectsEquals(f.internalFactory, ...)` on funcs | c: Java compares the factories by identity (PassListBuilder.contains → equals), and Go funcs have none |
| 4 | g_zstubs.go:3300 `Function_tfn_JSChunk_Set.apply(p0 any)` used only by Map_computeIfAbsent / mapComputeIfAbsent_ / computeIfAbsent (g_ConvertChunksToESModules.go:262, g_RescopeGlobalSymbolsRewriteCallback.go:236, g_JSChunkGraph.go:299) | func values | b: typed computeIfAbsent |
| 5 | g_ClosureExportTypeClassifier.go:139 `mapComputeIfAbsent_(tc_string, tc_p_..., this.topLevelNames, name, &Function_tfn_string_...{...})`: a native `map[string]*NameInfo` (line 6) whose value is boxed and unboxed through an adapter | Go map | b: `mapComputeIfAbsentT_(m, k, func(K) V)` |
| 6 | g_UnionType.go:86 `lambda_UnionType_matchesNumberContext_impl_1_(p0 any)`, given to `JSTypeIterations_anyTypeMatches(predicate func(any) bool, ...)` (g_JSTypeIterations.go:27), which calls `predicate(types.get(i).self())` over a typed `*ImmutableList[*JSType]` | rhino/jstype_helpers.go:348 `func(JSType) bool` | b: the table's "UnionType's erased predicates (17)" is a tfnparams gap, not an erased API |
| 7 | g_NodeUtil.go:3207 `NodeUtil_getCount(n, pred func(any) bool, ...)`: both callers pass named Match classes that have applyT_ (g_NodeUtil.go:3155, 3167) | node_util_traverse.go:297 `func(rhino.Node) bool` | b |
| 8 | g_AbstractCommandLineRunner.go:19 `inputsSupplierForTesting func() any` | cli/abstract_command_line_runner.go:156 `func() []*jscomp.SourceFile` | b. colls list -n gives "stays (122 declarations): converted from this.inputsSupplierForTesting()" as the first reason of a 122-declaration group; that needs checking, but it is the largest single list reason |
| 9 | g_ColorGraphBuilder.go:64 `to_LinkedDirectedGraph_LinkedDiGraphNode[*ColorGraphNode, any]` (28 `Graph..[.., any]` instantiations) | `[*ColorGraphNode, EdgeReason]` | b (graph E) |
| 10 | g_PassFactory.go:98 `equals(p0 any)` | pass_factory.go:156 `Equals(o any)` | a |
| 11 | JSDocInfo bag (g_0jsdocprops.go) | rhino/js_doc_info.go:80 `jsdocGet[T any](value any)` | a |
| 12 | g_SubtypeChecker.go:189 `JSTypeIterations_allTypesMatch(func(sub_ any) bool {...})` | typed | b (the 67 "function literal parameters" are mostly this) |

Estimate:

- **(d) 337.**
- **(b) ≈ 200:**
  - runtime-only adapter types: 67 of the 149 adapter types are given only to runtime functional
    APIs (computeIfAbsent/forEach/merge/compute, Comparator combinators, Suppliers.memoize,
    Iterables.*, Arrays.setAll, new_TreeSet_Comparator); that is ~110 slots;
  - func-literal parameters: ~40;
  - JSTypeIterations: 17;
  - graph E: ~15;
  - bridges: ~25.
- **(a) ≈ 330:** equals 119, the bag, Object fields, stubs, adapters stored in program fields.
- **(c) ≈ 65:** PassFactory, CmdLineParser options.

### Optional (1,451)

Prep 10 (tools/prep/10_find_null_strings/Convert.java) writes `@Nullable String` as
Optional<String>. j2go already writes every other nullable String as `string` with `nullstr_`
(rt_core.go:15; tools/j2go/RUNTIME.md:42). It already initialises String fields to `nullstr_` (284
`this.x = nullstr_` lines), and prep 10 gives a field without a value `= Optional.empty()`.

| our site | jsgo | class |
|---|---|---|
| g_PolymerClassDefinition.go:306 getInterfaceName | polymer_class_definition.go:448 `string` | b |
| g_RhinoErrorReporter.go:62 makeError(sourceName Optional[string]) | rhino_error_reporter.go:236 `string` | b |
| g_PolyfillUsageFinder.go:116 isDisjointFromIterator_2(ownerName) | polyfill_usage_finder.go:992 `string` | b |
| g_JSTypeRegistry.go:975 getReadableJSTypeName | jstype_js_type_registry_resolve.go:632 `string` | b |
| g_PolymerClassRewriter.go:493 createNewTypeExpressionForExtern | polymer_class_rewriter.go:574 `string` | b |
| g_ProcessCommonJSModules.go:103 getBasePropertyImport | process_common_js_modules.go:167 `string` | b |
| g_EnumType.go:284 getDisplayName | rhino/jstype_enum_type.go:272 `(string, bool)` | a |
| g_EnumElementType.go:314 getReferenceName | jstype_enum_element_type.go:194 `(string, bool)` | a |
| g_PrototypeObjectType.go:658 getReferenceName | jstype_prototype_object_type.go:515 `(string, bool)` | a |
| g_NodeUtil.go:3745 getBestLValueName | node_util_lvalues.go:778 `(string, bool)` | a |
| g_RewriteDynamicImports.go:28 init(alias Optional[string]) | rewrite_dynamic_imports.go:131 `base.OptString` | a |
| g_JSError.go:10 sourceName_ Optional[string] | js_error.go:44 `string`, :97 `base.OptString` | b/a |

Estimate:

- **(b) ≈ 950:** the jsgo `string` sites. jsgo writes "" there, which is a behaviour change, but
  `nullstr_` is exact, so they are (b) and not (c).
- **(a) ≈ 450:** the `(string, bool)` / OptString sites, and non-string Optionals
  (Node 11, []int8 6, ...).
- **(c) 0.**
- **(d) ≈ 50.**

Optional[string] also blocks colls. For example, g_MakeDeclaredNamesUnique.go:580/614
`MapOf[string, Optional[string]]` stays erased ("element type Optional[string] has no known
boxing", colls map -n).

### equals / hashCode (94)

| site | jsgo | class |
|---|---|---|
| g_RegExpTree.go:1212 LookaheadAssertion.equals_impl | regex/reg_exp_tree.go:631 `l.body.Equals(that.body)` | a |
| g_FunctionType.go:675-676 recursionUnsafeHashCode | jstype_function_type.go:758-759 | a |
| g_RegExpTree.go:2622, 763, 1363 | reg_exp_tree.go | a |
| g_HamtPMap.go:322 `Objects_equals(boxT_(t1.value_), boxT_(t2.value_))` | rhino/hamt_p_map.go:558 `hamtValueEquals(t1Val, t2Val)` typed | b |
| g_Node.go:1746 `Objects_equals(getter.apply(this.box_()), ...)` | node_equivalence_and_clone.go, per-property code | c |

Estimate: (a) ~80, (b) ~8 (`Objects_equals(boxT_..)` in generic code), (c) ~6. The reason holds:
jsgo has 351.

### for-each via iterator objects (108) / explicit Iterator (107)

| site | jsgo | class |
|---|---|---|
| g_InlineSimpleMethods.go:95 `foreach_(definitions)` (SetOf[Node] = erased Set; the body returns) | slices | b |
| g_SubtypeChecker.go:290 `foreach_(props)` | jstype_subtype_checker.go:513 `range keys.StringKeys()` | b |
| g_OptimizeParameters.go:124 `foreach_(functionNodes)` | optimize_parameters.go:418 `range functionNodes` | b |
| g_FixingErrorManager.go:83 `foreach_(this.multiFixes.keySet())` + `cast_[*JSError](it_3.v)` | refactoring/fixing_error_manager.go:172 | b |
| g_TypeInference.go:1391 lockstep `parameters.hasNext() && arguments.hasNext()` | type_inference_calls.go:322 `range inv.MustCallOrNew().Args()` | c |
| g_Compiler.go:2391 two lockstep iterators | compiler_sources.go:362 index loop | b (lockstep iterators over random-access lists → indices) |
| g_TypeCheck.go:1483 iterator handed to checkArgumentsMatchParameters | type_check_calls_props.go:126, a slice passed | c |
| g_ReferenceCollection.go:26 `iterator()` implementation | Iterable implementations | a |

Estimates:

- **foreach_ (108):** (b) ~85, (a) 5 (ranges' "used other than for the loop"), (c) ~18.
- **explicit iterators (107):** (a) ~45, (b) ~30, (c) ~30.

The 44 `it_N.v` casts (castcensus's "field declared Object: 44 v any") belong to this row.

### Erased collection constructors (175)

| site | jsgo | class |
|---|---|---|
| g_LinkedDirectedGraph.go:338, 380; g_FixedPointGraphTraversal.go:32; g_Graph.go:240 (element types in N/E) | typed slices in graph/ | b (colls needs a tconv for type parameters) |
| g_JSChunkGraph.go:374 `orderedInputs` | js_chunk_graph_deps.go:147 `[]*CompilerInput` | b |
| g_MakeDeclaredNamesUnique.go:614 (Optional[string] element) | | b (via the Optional rule) |
| g_SimpleTable.go:12 | | b (handover item 4) |
| g_ConvertChunksToESModules.go:263 | its own structs | c |
| g_AmbiguateProperties.go:486 new_BitSet | ambiguate_properties.go:217, its own bitset | a |
| g_JSDocSerializer.go:239 new_TreeSet | | a (natural order) |

Estimate: (b) ~90, (a) ~45, (c) ~40. colls -n names the large blockers:

- "result of a method of a generic type": list 92, set 21, map 10, immlist 7;
- "function literal used as a value": list 74, set 16, immlist 18, immset 18, map 9;
- "variable of a type parameter's type": 14 + 13;
- "equalsDyn_ ... takes a any": set 101.

The first three are mechanical: typed functional runtime variants, plus a tconv for type
parameters built on boxT_.

### String_* (126)

| site | jsgo | class |
|---|---|---|
| g_SuggestedFix.go:380 String_length, g_JsDocInfoParser.go:1617, g_CodePrinter.go:124 | jutil.StringLength (182 calls) | a |
| g_StripCode.go:83 String_toCharArray, g_Charsets.go:135 codePointCount | jutil.StringToUTF16 | a |
| g_ProfilingReport.go:59 `String_join_s("\n", Iterable(rows.asList()))`, g_CheckProvidesSorted.go:95 join of Iterables_transform | strings.Join of slices | b |
| String_format (19), e.g. g_DataFlowAnalysis.go:341 | jutil.StringFormat (34) | a |

Estimate: (a) ~105, (b) ~15 (String_join_s 13 + 2 over erased iterables), (d) ~5. The reason
holds, but its wording ("UTF-16 semantics") does not cover format or join.

### try/catch, defer, panic

| site | jsgo | class |
|---|---|---|
| g_NodeTraversal.go:117 catch_ | node_traversal.go:349 defer + recover | a |
| g_SubtypeChecker.go:113 defer (finally) | jstype_subtype_checker.go:173, no defer | c (jsgo dropped the finally) |
| g_AllType.go:65, g_LinkedUndirectedGraph.go:237, g_Color.go:25, g_StandardUnionFind.go:130 panics | panic / jutil.Check* | a |

Estimate: ~all (a). The reasons hold: jsgo has 58 recover, 304 defer, and 1,724 panic plus 1,882
Check*.

## 3. Rules and tools ranked by sites removed

| rank | rule / tool | removes (estimate) | effort |
|---|---|---|---|
| 1 | **optstr** (postflip; or reverse prep 10's Optional mode, which needs a re-flip). Mapping: `Optional[string]`→`string`; `Optional_of[string](x)`/`ofNullable`→`x`; `Optional_empty`→`nullstr_`; `isPresent()`→`!= nullstr_`; `isEmpty()`→`== nullstr_`; `get()`→`strget_(x)` (NoSuchElement on nullstr_ keeps the exception path); `orElse(y)`→`strOr_(x, y)`; `optEquals_`→`==`; `box_()`→`strobj_` | Optional row 1,451 → ~75 (1,376 type spellings, ~2,700 calls); unblocks colls on Optional elements | medium: one go/types tool, ~600-900 lines, no runtime change, behaviour-neutral (the REFLIP.md text at line 1128 already says so) |
| 2 | **typed functional runtime variants + rewrite** (`mapComputeIfAbsentT_(m, k, func(K) V)`, `forEachT_`, `mergeT_`, `computeT_`, typed Comparator combinators (`comparingT_`, `comparingIntT_`, `thenComparing*T_`), `memoizeT_`, typed Iterables transform/filter/concat as iter.Seq, typed `Iterable_forEach`). The rewrite turns `&X_tfn_Y{f}` into `f` | 67 adapter types, ~250 of 587 adapter literals, any ~110, casts ~45-60, func-literal parameters ~40; unblocks colls' "function literal / method used as a value" groups | medium: ~25 runtime functions and a tcmp-like tool |
| 3 | **typed views of typed containers**: `m.asImm().entrySet/keySet/values/get` on *TMap / *ImmutableMap → typed entries (`tentry_[K,V]` with getKey()/getValue()) or `m.All()`; `immElems_(l.asImm())` → `l`'s typed elements; `immCursorOf_(l.asImm())` → `immCursorT_[T]` | casts ~100 (64 + 10 + ~21 + ~15), unboxings ~40 | small |
| 4 | **tconv for type parameters** in colls (box via boxT_, unbox via the assertion) | the "generic type" / "type parameter" groups: ~40 constructors, ~60 casts | medium-high |
| 5 | **implsplit bridges' callers typed**: natural-order sorts and TreeSets of typed lists use compareToT_, then the dead bridges are deleted (the deadtfn pattern) | 28 bridges, ~30 casts, ~55 any | small |
| 6 | **tfnparams for func literals / named lambdas whose parameter is cast on entry**: JSTypeIterations (g_JSTypeIterations.go:18-41), SubtypeChecker.go:189/195, UnionType.go:86-142, NodeUtil_getCount (g_NodeUtil.go:3207); func-typed fields by Java's type argument (inputsSupplierForTesting) | casts ~25, any ~40, unboxings | small |
| 7 | **Graph/DiGraph keep E** (j2go rule or a postflip on the graph files; wildcards `DiGraph<N, ?>` become a type parameter) | casts ~20, 28 `[.., any]` instantiations, any ~15 | medium |
| 8 | **typed proto `buildT_()`** (tools/protogen emits it; a castfold-like rewrite) | casts ~25 | small |
| 9 | **generic methods → package generic functions** (runInCompilerThread, as jsgo's Compiler_RunInCompilerThread[T]); typed Iterables_getOnlyElement / getFirst / getLast / first_ on typed lists | casts ~25 | small-medium |
| 10 | **foreach_ with a returning body → range over the collection's element slice** (the immElems_ pattern for slice-backed erased lists and sets; no closure, so the range-over-func cost does not apply) | 105 cursor loops (the row's Java-ism); the element casts stay unless rank 4 types the collection | small; needs one measurement |

Together, ranks 2-9 remove about 300-350 casts: 850 → ~500-550, below jsgo's 648, before the
like-for-like correction of section 1. They remove about 230 any slots: 597 comparable → ~370,
against jsgo's 267. Rank 1 alone removes 95% of the Optional row.

## 4. Reasons in the table that do not hold

1. **Optional:**
   - "It costs nothing" is a performance argument. It is not a reason for keeping a Java-ism.
   - "Needs every Optional field proved assigned" does not hold, for two reasons. j2go already
     initialises String fields to `nullstr_` (284 sites), the exact null of a String. Prep 10
     already gives field declarations `= Optional.empty()`.
   - "jsgo writes '' for null" is half the story. jsgo also uses `base.OptString` (542 sites)
     and `(string, bool)` (243 functions). Where it does use "", the exact mechanical form is
     `nullstr_`.
   - Verdict: about 950 sites are (b).
2. **Lambda adapters "named where a lambda meets an erased interface: javac's bridge":**
   - 67 of the 149 adapter types meet only runtime functional APIs that a typed variant
     absorbs. g_ClosureExportTypeClassifier.go:139 boxes and unboxes through an adapter into a
     native Go map.
   - The row also misses 587 adapter literals (section 1.4).
   - PassFactory's 175 (identity, PassListBuilder.contains) do hold, and jsgo changed behaviour
     there (pass_factory.go:156-168).
3. **"198 Object results of the runtime (erased collections' elements...)":** about 110 of them
   are erased views (`asImm()`) of containers that are already typed (FunctionInjector.go:146-149,
   RhinoErrorReporter.go:135, FunctionType.go:950), not erased collections.
4. **"UnionType's erased predicates over its alternates (17)":** the predicates' runtime
   (JSTypeIterations, g_JSTypeIterations.go:18-41) iterates a typed `*ImmutableList[*JSType]` and
   boxes each element with `.self()`. This is a tfnparams gap. jsgo: `func(JSType) bool`.
5. **"graphs' edge values E" (any) and the edge-value casts:** E was dropped by j2go, not by
   Java. jsgo keeps `DiGraph[N, E]`, and Java has only 2 wildcard uses.
6. **for-each "a range-over-func loop is slower there":**
   - It is applied to all 105 loops without a per-loop measurement.
   - A range over the element slice (as immElems_ already does for immutables) has no closure.
   - The cursor's "re-read of the list" only differs from a snapshot when the body modifies the
     list, which is a ConcurrentModificationException path in Java.
7. **Casts total "jsgo ~726":** the figure is 648 (section 1.1), and our comparable count is about
   1,280, not 850 (section 1.2).
8. **"any slots ... none typeable" (objslots):** this reads per slot. The (b) items above are
   whole-group changes (runtime variants, kept type parameters) that objslots' single-slot test
   cannot see.
9. **String_* "UTF-16 semantics":** right for charAt, substring and length. It does not cover
   String_format (Java format semantics, which jsgo keeps) or String_join_s over erased iterables
   ((b)).

These hold, as checked against samples:

- the self() downcasts (except g_ConformanceRules.go:1040, an enum upcast with no Java cast);
- the JSDocInfo bag (jsgo has 90 jsdocGet[T]);
- equals/hashCode;
- panic, defer and recover;
- threads, StringBuilder, reflect and unsafe;
- Guava immutables.
