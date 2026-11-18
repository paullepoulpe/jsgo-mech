# Pressure test: what is still erased in raw j2go output, and must it be?

Base: origin/main-r8-typed 42520823 (scratch worktree /home/user/jm-ptest, nothing committed or pushed).
Raw census at base: boxing 10,894, class casts 3,154, interface casts 2,430, unboxing 776, any declarations 1,893.

Short answer: no, most of it need not stay erased. Five translator rules prototyped here take boxing to 5,990 (-45%),
interface casts to 645 (-73%), class casts to 2,775 (-12%), unboxing to 682 (-12%), with byte-identical output and
CPU time level or better. What is left is dominated by causes with a known clean fix (Guava immutables, for-each over
erased collections, records, CharSequence, program generics erased for one wildcard). The part that Java itself makes
dynamic is small: about 330 explicit casts / instanceof, 182 Object declarations, heterogeneous format varargs.

## 1. Method

- Report-only instrumentation in the scratch j2go (in j2go-ptest.patch):
  - `J2GO_PTEST=<file>`: every `Ctx.coerce` that adds a box, cast or unbox writes one row: counts, javac kind and type
    of source and target, Java file:line, the emitter frames, the callee and parameter type if it is an argument, the
    callee and declared result if it converts a call's result. 15,111 rows; sums 10,950 box / 3,228 class cast /
    2,287 icast / 743 unbox, against census 10,894 / 3,154 / 2,430 / 776 (the census also counts runtime-shaped
    text such as `any(this)` outside coerce and `.(int32)`; the log counts `boxT_`).
  - `J2GO_PTEST_DECLS=<file>`: every source declaration whose Go type is or contains `any`, with its Java type.
- `classify.py` maps each row to a cause (`python3 classify.py` = table; `python3 classify.py <letter> 20` = callees
  and examples). `sites.tsv` = base, `sites_after.tsv` = with prototypes P1-P4, `decls.tsv` = declarations.

## 2. Cause table (base output; box / class cast / icast / unbox)

Verdicts: (a) clean translator rule, (b) post-flip only, (c) inherent in the Java. "done" = prototyped here.

| # | Cause | box | cast | icast | unbox | Verdict |
|---|---|---|---|---|---|---|
| U | class -> interface it implements (a static upcast in Java, no checkcast) | 1,648 | 14 | 1,765 | 0 | (a) done, P1 |
| E1 | argument of an erased library generic (parameter is a type variable) | 2,586 | | | | (a): 463 done P2; ~1,720 Guava immutables; ~400 java.util / functional |
| E1r | result of an erased library generic | | 767 | 88 | 208 | (a): 267 done P2; rest follows typed collections |
| F | for-each over an erased collection (source as any + element cast) | 1,003 | 705 | 139 | 225 | (a) with typed immutables / remaining collections |
| O1 | Object / Object... message and format arguments | 1,475 | | | | (a) done for Preconditions, P3+P4; String.format varargs (c) |
| R | record equals / hashCode / toString generated through Object | 685 | | | | (a), not prototyped |
| K | result declared on a supertype (no covariant result in Go) | 243 | 643 | | | (a): library half done P5; program half needs forwarders |
| L | lambda / method-reference adapter of an erased functional interface | 209 | 269 | 153 | 12 | upcasts done P1 (66 / 269 / 10 / 12 left); rest (a) per API |
| S | String passed where CharSequence / Comparable is declared | 475 | | | | (a), runtime string variants; literals done P3 |
| T | typed-collection element converters (glue to the erased view) | 221 | 223 | 25 | 1 | stays while erased code sees the collection; 272 converters, each emitted once |
| E2 | argument / result of a program generic that GenScan erased | 466 | 255 | 14 | 19 | (a) mostly, see 2.2 |
| O2 | Object parameter: equals / Objects.equals / hash / identical | 390 | | | | (c) by signature for equals(Object); ~110 (a) |
| D | value assigned / returned / concatenated as Object | 560 | | | | ~370 string concatenation (a); rest (c) |
| C | receiver boxed for an Object method resolved to java.lang.Object | 321 | | | | (c): dynamic type decides; cheap |
| Y | unboxing a boxed primitive (Integer/Double/Boolean values) | | 12 | | 248 | (a) candidate (nullable value type), not prototyped |
| J | explicit cast written in the Java source | | 188 | 5 | 4 | (c) |
| X | instanceof pattern / catch / switch pattern binding | 5 | 145 | 13 | 4 | (c) |
| O4 | Object parameter of a program method | 184 | | | | (c) mostly (property bag, table lookup by Object) |
| O3 | Object key of a library collection lookup (get / contains / remove) | 164 | | | | (a), follows typed collections |
| V | value of a kept type parameter meets an erased slot (boxT_/unboxT_) | 148 | 1 | 7 | 22 | (a), follows E1/E2 |
| G | bridge method: this as the interface | 49 | | 58 | | done P1 (9 / 18 left) |
| - | other | 122 | 6 | 20 | | |

Examples (Go line is in the base output unless marked "after"):

- U: AliasStrings.java:106 `NodeTraversal.traverse(compiler, root, this)` -> `icast_NodeTraversal_Callback(any(this))`;
  after: g_AliasStrings.go:40 `NodeTraversal_Callback(this)`. PolyfillUsageFinder.java:255 ->
  `icast_NodeTraversal_Callback(obj_(new_PolyfillUsageFinder...))`. ScopedAliases.java:394 `icast_Collection(obj_(ImmutableSet_of_Object(...)))`.
- E1: g_AbstractCommandLineRunner.go:705 `outputFilesBuilder.put(strobj_(name), strobj_(filename))` (ImmutableMap.Builder<K,V>.put);
  g_InvalidatingTypes.go:78 `ImmutableList_of_...(obj_(JSTypeNative_FUNCTION_FUNCTION_TYPE), ...)` (InvalidatingTypes.java:196).
- E1r: g_Compiler.go:1222 `cast_[*CompilerInput](this.inputsById.get(obj_(id)))` (Compiler.java:1862);
  g_JSTypeRegistry.go:1120 `to_JSType(e.getValue())` (JSTypeRegistry.java:1996).
- F: g_CheckJSDoc.go:47-49 `it_1 := foreach_(any(this.compiler_.getComments(fileName)))` ... `cast_[*Comment](it_1.v)`.
- O1: NodeUtil.java:973 `checkState(..., node)` -> `Preconditions_checkState_boolean_Object(..., node.box_())`;
  after: g_NodeUtil.go:28 `Preconditions_checkState_boolean_Object_H(templateLitString.isTemplateLitString(), templateLitString)`.
- R: g_SourceMapSection.go:75 `Objects_equals(obj_(this.type__), obj_(o.type__))`, :80 `Objects_hashCode(obj_(this.type__))` (SourceMapSection.java:55).
- K: QualifiedName.java:70 `cast_[*ImmutableList](obj_(builder.build()))`; g_ConstCheck.go:40 `cast_[*Var](t.getScope().getVar(name).self())` (ConstCheck.java:96).
- S: g_AbstractCommandLineRunner.go:438 `String_replace(zipEntryName, strobj_(filename), strobj_(rootFilename))`.
- E2: ConformanceRules.java:2640 `SimpleListMultimap.put("HTMLAnchorElement", "a")` -> `put(strobj_(..), strobj_(..))`; g_JSDocInfo.go:17 `to_JSDocInfo_Property(entry.getKey())`.
- O2: g_PeepholeFoldConstants.go:704 `n.getColor().equals(obj_(StandardColors_NUMBER))`.
- D: g_Compiler.go:243 `"..." + objstr_(obj_(options.getDependencyOptions()))`.
- Y: g_PeepholeFoldConstants.go:162 `var value float64 = unbox_[float64](result)`.

### 2.1 Declarations of type any

The census's 1,893 is mostly generated code: 452 parameters of lambda adapters and dispatch stubs (`p0 any`), 405 in
the generated support files (icast_ and converter functions), 344 `to_X / is_X / X_of_ (v any)` helpers, 47
`equals(Object)`. About 650 are real declarations. javac-side (decls.tsv), 481 source declarations are `any`:

- 182 `Object` in the Java source: 104 parameters (47 are `equals(Object)`), 56 results (37 are `visitUntyped`), 14
  locals, 8 fields. I checked all 8 fields; each is a real union, so (c): GlobalNamespace.java:1253 `refsForNode`
  ("either null, a Map<Node, Ref> or a singleton Ref"), Node.java:1690 `jstypeOrColor`, Node.java:1713 `payload`,
  Node.java:344 `objectValue`, Property.java:162 `name`, JSDocInfo.java:389 `propertyValues`,
  ClosureBundler.java:43 `minifier`, DisambiguateProperties.java:457 `value`. None is an Object that holds one class.
- 180 boxed primitives (Boolean 64, Double 54, Integer 51, ...): nullable numbers, see rule 6 below.
- 110 erased type variables, 8 CharSequence, 1 Void.

Another 682 declarations have `any` only inside a type argument: 122 `BoxedOf[int32]`-style, 74 a type variable, 63
`Object`, 50 a wildcard, 373 mixed (many are `Outcome<Void>` -> `*Outcome_base[any]`).

### 2.2 Program generics that GenScan erased (E2), by its own reasons

- SimpleListMultimap (203 args): "K, V erased: a wildcard (SimpleListMultimap.java:173)". One wildcard mention
  erases the class. Same shape as the existing "wildcards written for brevity" rule: (a).
- SimpleSetMultimap (115 args): "holds an inner class". A translator limitation, not Java semantics: (a).
- JSDocInfo.Property (57 results + 41 args): "T erased: a wildcard (JSDocInfo.java:82)" -- a heterogeneous property
  map keyed by `Property<?>`: (c) for the map, (a) for typed accessors.
- AbstractScope / AbstractVar (getVar 125, getScope 62, getParent 39, ...): "bounded by a class". Go has no class
  bound, but each subclass binds the variable (`Scope extends AbstractScope<Scope, Var>`), so a typed forwarding
  method in the subclass moves ~250 call-site casts into about ten one-line methods: (a), only for method groups
  that no Go interface names.
- JSType.visit (28): a generic virtual method `<T> T visit(Visitor<T>)`; Go has none: (c) for Go.

## 3. Prototypes

All in one patch behind `J2GO_PT=upcast,upcastlib,nn,lit,msg,alias` (default off: output is byte-identical to base
without the variable). Runtime additions in go/closure/rt_ptest.go, 7 lines changed in rt_base.go.

| | Rule | box | icast | cast | unbox |
|---|---|---|---|---|---|
| P1 `upcast` | A program class without subclasses becomes an interface it implements as Go converts it. `this` and `new_C(...)` are never null: `I(this)`. Any other value goes through a generated `C_as_I(v)` that keeps null as the nil interface. Guard: `Ctx.implementsInGo`. | -1,005 | -1,005 | | |
| P1 `upcastlib` | The same for runtime classes that cannot be subclassed (ArrayList, ImmutableList, typed-collection views -> List / Set / Iterable / Map / Collection). The Go compiler checks each pair; all 179 generated functions compiled first time. | -760 | -760 | | |
| P2 `nn` | `<T> T checkNotNull(T, ...)` / `requireNonNull(T, ...)` are Go generic functions of the argument's static type (`_T[T comparable]`, `_Str` for String): no box in, no cast out. | (with P3) | -20 | -136 | -94 |
| P3 `lit` | A string literal or `new_C(...)` passed for an Object parameter is passed as it is (never null, Go converts). | -2,096 (P2+P3) | | | |
| P4 `msg` | Message arguments of Preconditions.checkState / checkArgument / checkNotNull are passed at their own type; the runtime canonicalises (`canon_`: nil pointer and null string -> null, `self()` for a class with subclasses) only when the check fails. A handle (Node) as the single message argument calls a generic `_H[M handle_]` variant and is boxed on failure only. | -800 (205 + 595) | | | |
| P5 `alias` | Two library classes that the runtime declares as one Go type (`type ImmutableList = ImmutableCollection`) convert as the value itself; today's `cast_[*ImmutableList](obj_(b.build()))` asserts a type to itself. | -243 | | -243 | |
| all | | 10,894 -> 5,990 | 2,430 -> 645 | 3,154 -> 2,775 | 776 -> 682 |

Gate (`flock gate.lock bash tools/prune/verify.sh`, flags exported so the selftest uses them):

| Set | selftest | runtime tests | checks / bundle / downlevel / advanced |
|---|---|---|---|
| P1 | PASS (ran without the flag in this run) | ok, 16 | MATCH x4 |
| P1+P2+P3 | PASS | ok, 16 | MATCH x4 |
| P1..P4 | PASS | ok, 16 + 2 new (rt_ptest_test.go: failure messages and null handling equal to the boxed calls) | MATCH x4 |
| P1..P5 | PASS | not run | not run: the disk filled (link: "no space left on device", 350 MB free, nothing in the Go cache older than 180 min to trim). `go build ./closure` of the P5 output succeeded before that. |

The e2e gate does not make checks fail, so P4's failure path is covered only by the two unit tests.

Time (fb.py, alternating order, under the lock, no PGO; other agents were running, load 1.5 to 8.6). CPU seconds:

- bundle, 4 rounds: base 40.52 / 42.57 / 42.33 / 41.28 (mean 41.68); P1 40.03 / 41.84 / 40.97 / 39.43 (40.57, -2.7%);
  P1-3 41.23 / 40.09 / 40.07 / 41.03 (40.61, -2.6%); P1-4 39.92 / 39.66 / 41.09 / 40.02 (40.17, -3.6%).
  Wall means: base 37.5, P1 36.7, P1-3 35.7, P1-4 35.4.
- typed-full, 2 rounds: base 86.59 / 84.95 (85.77); P1-4 84.87 / 86.15 (85.51, -0.3%).

So no prototype is slower; bundle looks 2.5-3.5% better but with run-to-run spread of about 2% at this load I would
only claim "level or better". P5 is not measured. Allocations were not measured (disk).

## 4. Rules worth adding next, ranked

1. **Static upcast of a leaf class to an interface (P1)**: -1,765 boxes, -1,765 interface casts. Prototype gated and measured. Left after it: 183 icasts (class with subclasses: `icast_I(x.self())`; program classes whose Go type lacks a method of the interface; generic classes).
2. **Non-null literal / new arguments, generic checkNotNull, Preconditions message arguments (P2-P4)**: -2,896 boxes, -136 casts, -94 unboxes. Gated and measured. The hand port did the same (`CheckNotNullMsg1[A any](n Node, template string, p1 A)`, generic `jutil.CheckNotNull`).
3. **Typed Guava immutables** (ImmutableList / Set / Map and builders as typed collections): about 1,720 argument boxes, ~340 for-each sources with their element casts, ~170 `get` casts, and the 243 + 243 of P5 become moot. Not prototyped (the largest remaining item; main-r8-typed's own commit message names it next).
4. **Records typed** (equals by `==` or a typed equals, typed hash, typed toString): -685 boxes; cold code. String fields are already compared directly.
5. **String for CharSequence**: 475 sites (about 200 literal ones already bare with P3). Needs string variants of about 12 runtime functions (replace, contains, Pattern.matcher, Appendable.append, String.join, Splitter). Passing the string raw is not safe as is: program methods with CharSequence parameters and `append_CharSequence` test for nil, not for the null string.
6. **String concatenation of a typed value** (`"" + x` -> `objstr_(obj_(x))`): ~370 boxes with a typed helper.
7. **Program generics erased for one wildcard or an inner class** (SimpleListMultimap, SimpleSetMultimap): ~320 boxes; and typed forwarders for class-bounded variables (AbstractScope / AbstractVar): ~250 casts.
8. **Nullable primitive as a value type** (as Optional became one): 180 declarations, ~250 unboxes, part of 506 `any(prim)` boxes; removes an allocation per boxed double. Needs care with `==` on boxed values. Not sized further.
9. **Library aliases (P5)**: -243 / -243, but only until rule 3 lands.

Stays, with the Java that requires it: explicit casts and instanceof (about 350; e.g. Parser.java:471 `(LiteralToken) eat(TokenType.STRING)`), the 182 Object declarations (unions, `equals(Object)`, `visitUntyped`), `String.format` / `LogFile.log` varargs (LoggerErrorManager.java:64 mixes int and double), Object-keyed lookups in program tables (SimpleTable.get(Object)), Object methods resolved to java.lang.Object on a class with subclasses.

## 5. Files (this directory)

- report.md (this), ptest-all.patch (everything), j2go-ptest.patch (translator: instrumentation + P1-P5), runtime-ptest.patch (rt_ptest.go, rt_ptest_test.go, rt_base.go)
- sites.tsv, sites_after.tsv, decls.tsv, classify.py, classify_after.py, an.py, run0.log (J2GO_GENERICS=1 J2GO_COLLS=1 at base), census_after.txt
- gate_p123.log, gate_p1234.log, gate_p12345.log (the failed run), fb.log
- binaries kept: proto/m_pt0 (base), proto/m_pt1234
