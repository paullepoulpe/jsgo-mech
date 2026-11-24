# classify6.py: a verdict for every census match of tree6h (JAVAISMS6.md).
# inputs (cwd): sc.tsv (sites.tsv of jclass + ctx.tsv's enclosing function, enclosing calls, type),
#               box_joined.tsv (boxjoin.py), matches.tsv, tree/go/closure (source lines)
# output: verdicts.tsv: row file line verdict label ; and a per-row table on stdout.
# usage: python3 classify6.py [-v ROW]   (-v: list the sites of a row with their labels)
import collections, re, sys

SRC = "tree/go/closure/"
_lines = {}
def line(f, l):
    if f not in _lines:
        _lines[f] = open(SRC + f).read().split("\n")
    return _lines[f][int(l) - 1]

names = ["row", "file", "line", "hot", "sub", "detail", "extra", "matched", "fn", "calls", "typ"]
S = [dict(zip(names, l.rstrip("\n").split("\t"))) for l in open("sc.tsv")]
for d in S:
    for k in names:
        d.setdefault(k, "")
    d["src"] = line(d["file"], d["line"])

# the erased runtime library surface (static functions of the runtime whose Java signature is generic)
API = re.compile(r"^(Iterables_|Iterators_|Lists_|Sets_|Maps_|Collections_|Map_|Iterable_|List_|Collection_|SequencedCollection_|"
                 r"MoreObjects_firstNonNull|Io_listFiles|CacheLoader_from|newTSortedSetC_|newTSortedMapC_|JString_join|Object_clone|"
                 r"Comparator_|Ordering_|Streams_|Splitter|.*\.split_s$|Iterables\b)")
# values of library types the typed-interfaces rule did not reach (the 34 XOf aliases and the Erased* Guava types)
LIB = re.compile(r"(Erased|ImmutableSorted|ImmutableSetOf|ImmutableListOf|ImmutableMapOf|Optional_base|LoadingCache|CacheBuilder|PeekingIterator|"
                 r"Deque|Queue|PriorityQueue|IdentityHashMap|EnumSet|EnumMap|TreeMap|Ordering|ThreadLocal|ConcurrentMap|ImmutableCollection|"
                 r"ListOf|MapOf|IterableOf|CollectionOf|SetOf)")
PROTO = re.compile(r"\.get\w+List\(\)|toBuilder|ProtocolStringList")

def calls(d):
    return [c.rsplit("#", 1)[0] for c in d["calls"].split(" < ") if c]

def first_api(d):
    for c in calls(d):
        base = c.split(".")[-1] if "." in c and not c.startswith("*") else c
        if API.match(c) or API.match(base):
            return c
    return None

def castlike(d):
    """rows 10, 11, 16: interface casts, class casts, unboxing"""
    s, det, ex, src = d["sub"], d["detail"], d["extra"], d["src"]
    if s.startswith("NOT CODE"): return "not a site", "comment"
    if s.startswith("helper declaration"): return "not a site", "helper declaration"
    if s.startswith("cast function used as a value"):
        return "not a site", "cast function as a value (" + ("tconv_ unbox field" if "tconv_" in s else "arrmap_") + ")"
    if s.startswith("inside a cast helper"): return "not a site", "fallback inside a gcast_ helper"
    if "field of tconv_" in det or (d["file"] == "g_0casts.go" and "unbox:" in src):
        return "not a site", "inside the unbox function of a tconv_ descriptor (goes with the descriptors, section 3)"
    g = "GUARDED" in ex
    if "catch_" in det or (g and ("Exception" in ex or "Throwable" in ex)):
        return "DONE", "caught exception asserted after its catch test"
    if g: return "DONE", "cast after an instanceof test of the same value"
    if s.startswith("self()"): return "DONE", "self() of a typed value: down/cross-cast in a class hierarchy"
    if det in ("Node.getProp", "Node.getFeatureSet", "JSDocInfo_Builder.getProp", "JSDocInfo_Builder.getPropWithDefault") or "valueT_" in det:
        return "DONE", "Node / JSDocInfo property bag read (jsgo: GetProp any, node_cast / jsdocGet)"
    if s.startswith("`.(bool)`") or "comma-ok" in s: return "DONE", "comma-ok test of an Object"
    if det == "Annotatable_getAnnotation": return "REASON", "graph annotation (jsgo: flow states in a side slice, by hand)"
    if det in ("Node.jstypeOrColor_get_", "Node.payload_get_", "AbstractCompiler.getTypedScopeCreator", "NodeTraversal.getScopeCreator",
               "Node.getStaticSourceFile", "AbstractCompiler.getScopeCreator"):
        return "REASON", "Object or interface result in Java (" + det + ")"
    if det.startswith("FlowScope.") or "withSyntacticScope" in det or "joinFlow" in det or "HamtPMap" in det or "equivalent" in det:
        return "DONE", "Java downcast of an interface value to its class (jsgo asserts too)"
    if "boxT_" in s: return "REASON", "boxT_ of a type parameter (Go cannot assert a type-parameter value)"
    if "flag_" in det:
        return "RULE R4", "generic program class with an erased type parameter (AbstractCommandLineRunner.FlagEntry<T>.flag)"
    if det in ("SourcePosition_rhino.getItem", "JSDocInfo_NamePosition.getItem", "JSDocInfo_TypePosition.getItem") or "setItem_impl" in det:
        return "RULE R4", "generic program class with an erased type parameter (SourcePosition<T>)"
    if "clone_impl" in det or "equalValues_impl" in det or "getTypeExpressions_Object_impl" in det:
        return "DONE", "JSDocInfo.Property value methods take Object (jsgo EqualValues(left any, right any), rhino/js_doc_info.go:259)"
    if "Property.name_" in det or "refsForNode" in det or "LogsGson_write" in det or "getNodeRootFromScopeObj" in det \
            or det in ("sliceAt_", "slicePeekT", "slicePeekTop_", "TEntry.getValue", "TMap.Get") or "TEntry.ge" in det or det == "TMap":
        return "REASON", "Object slot holding values of two or more types in Java (" + det + ")"
    if "compareTo" in det or det.endswith(": compare"):
        return "RULE T4", "bridge of Comparable/Comparator implemented by a class (erased callers: the runtime's sorts)"
    if det.endswith(": equals"): return "DONE", "equals(Object) parameter (jsgo Equals(any))"
    if det.endswith(": add") or det.endswith(": load") or det == "Source_Lazy.get":
        return "RULE T4", "bridge of a class implementing a library interface (" + det.split(": ")[-1] + ")"
    if PROTO.search(src) and ("foreach_" in det or "toBuilder" in det or "ProtocolStringList" in det or "getRequirementList" in src):
        return "RULE T5", "protobuf list accessor or builder returning an erased value"
    if "element of a slice or map of Objects" in s or det == "[]any":
        return "RULE T1", "element of toArray's []any (typed toArray)"
    if "map_EntryAs_" in src and re.search(r"List_sort|Collections_sort", d["calls"]):
        return "REASON", "typed sort of typed entries (REFLIP6 'typed sorts': wrappers break K(box(x)) == x; a rule was tried and dropped)"
    if "ImmutableMap_Builder[*Node_Prop, Function[any, any]]" in d["calls"]:
        return "RULE T7", "func type as a type argument of a typed container (descriptor tc_Function_any__any_)"
    if det.startswith("foreach_(") or det.startswith("elems_("):
        if re.search(r"foreach_\(obj_\(", src) or re.search(r"foreach_\(obj_\(", "".join(line(d["file"], int(d["line"]) - k) for k in (1, 2, 3))):
            return "RULE T6", "for-each over a typed value boxed to Object (a program class implementing Iterable<T>, EnumSet)"
        prev = "".join(line(d["file"], int(d["line"]) - k) for k in (1, 2, 3))
        if re.search(r"foreach_\(\w+\.get\w+List\(\)\)|getRequirementList|getValueList", prev):
            return "RULE T5", "protobuf list accessor returning an erased List"
        m = re.search(r"foreach_\((\w+)", prev)
        if m and API.match(m.group(1)) or "split_s" in prev or "Io_listFiles" in prev:
            return "RULE T1", "erased runtime library function: " + (m.group(1) if m else "?") + " (for-each over its result)"
        if "getInEdges" in prev or "getOutEdges" in prev:
            return "REASON", "Java wildcard List<? extends DiGraphEdge<N, E>> (Go has no variance; jsgo's graph is concrete, by hand)"
    if re.search(r"ImmutableSorted|ImmutableList_sortedCopyOf|Optional_base|Ordering", d["calls"]) and "func literal" in s:
        return "RULE T2", "converting closure for an erased Guava type (sorted immutables, Optional.transform)"
    if det in ("Map.get",) and "MapOf" in src or "Timeline" in d["file"] and "Map.get" in det:
        return "RULE T2", "value of an erased library type (MapOf alias)"
    if "ProtocolStringList" in det: return "RULE T5", "protobuf list accessor returning an erased List"
    if d["file"] == "g_0casts.go" and "func literal" in s:
        return "RULE T7", "converting closure of a func-typed conversion in g_0casts.go"
    if det.startswith("elemsT_("): return "DONE", "cross-cast of a typed interface value to another interface it implements"
    if re.match(r"(Optional_base|PriorityQueue|IdentityHashMap|ThreadLocal|ErasedImmutable|ImmutableCollection|LoadingCache|PeekingIterator|Deque)\w*\.", det):
        return "RULE T2", "value of an erased library type (" + det.split(".")[0] + ")"
    if det.startswith("Map_Entry.get"):
        return "RULE T2", "entry of an erased library map (ImmutableSortedSet / erased entries)"
    a = first_api(d)
    if not a and API.match(det): a = det
    if not a and s.startswith("Object result of a runtime function") and det: a = det
    if a: return "RULE T1", "erased runtime library function: " + re.sub(r"\[.*", "", a)
    if det == "lgBox_": return "RULE T1", "erased runtime library function: SequencedCollection_getFirst"
    if LIB.search(det) or LIB.search(src) and ("immElems_" in det or "Map.get" in det):
        return "RULE T2", "value of an erased library type (" + re.sub(r"\[.*", "", det) + ")"
    if det.startswith("Map_Entry.get") or det.startswith("foreach_(") or det.startswith("immElems_") or det.startswith("elems_("):
        return "UNSPLIT", "erased element/entry: " + det
    if "func value" in s: return "RULE T1", "result of a func value typed func() any (converting closure)"
    if "Object field" in s or "field declared Object" in s: return "REASON", "Object field read"
    return "UNSPLIT", s + " :: " + det


BOX = {}
for l in open("box_sc.tsv"):
    f = l.rstrip("\n").split("\t")
    BOX[(f[1], f[2], f[7], f[8])] = f  # file line matched fn -> joined row (may collide; values identical in kind)
_boxq = collections.defaultdict(list)
for l in open("box_sc.tsv"):
    f = l.rstrip("\n").split("\t")
    _boxq[(f[1], f[2])].append(f)

def boxing(d):
    f = _boxq[(d["file"], d["line"])].pop(0)
    cat, sink, vt, calls = f[13], f[14], f[16], f[9]
    src = d["src"]
    if cat.startswith("NOT CODE"): return "not a site", "comment"
    if (d["file"], d["line"]) in (("g_CrossChunkCodeMotion.go", "533"), ("g_InlineAndCollapseProperties.go", "1078"), ("g_InlineAndCollapseProperties.go", "1290")):
        return "RULE E-tostr", "objstr_(obj_(x)): the existing tool tostr rewrites it to tostr_(x) on this tree"
    if re.search(r"FlagEntry|addMultiFlagValue", sink): return "RULE R4", "generic program class with an erased type parameter (AbstractCommandLineRunner.FlagEntry<T>)"
    if re.search(r"CmdLineParser_Option|LogsGson_toJson|new_Property#|addNewEntry|Logger\.log", sink):
        return "REASON", "Java signature takes Object (args4j option values, LogsGson, Property key, TypeTransformation entries, Logger)"
    if "ThreadLocal" in sink: return "RULE T2", "argument of an erased library type's method (ThreadLocal)"
    if cat == "runtime method parameter (BigInteger)" or cat == "runtime method parameter (ComparisonChain)" or cat == "runtime method parameter (Enum)":
        return "RULE T1", "runtime method whose Java signature is typed or overloaded (BigInteger.compareTo(BigInteger), ComparisonChain.compare(Comparable), Enum.compareTo(E))"
    if cat == "variadic runtime argument": return "DONE", "variadic runtime argument (Strings_lenientFormat, String_format: ...any as jsgo)"
    if cat == "variadic generated argument": return "DONE", "variadic generated argument (Java varargs of Object: LogFile.log, Msg.format, Parser.report)"
    if cat == "comparison": return "DONE", "identity comparison of values of two static types (needs an interface in Go)"
    if cat.startswith("element of a composite"): return "DONE", "element of a []any literal"
    if cat.startswith("conversion of a value that is already") or cat.startswith("type assertion of a box"):
        return "DONE", "any(x).(T) on an interface value (Go's instanceof on an interface)"
    if d["file"] == "g_0casts.go" and ("box:" in src or "tconv_" in f[8] or f[8].startswith("tc_")):
        return "not a site", "box function of a tconv_ descriptor (goes with the descriptors)"
    if re.search(r"populateProp|setProp|putProp|populatePropEntry|putPropEntry|payload_set_|jstypeOrColor_set_", sink):
        return "DONE", "Node / JSDocInfo property bag store (jsgo PutProp(prop, value any))"
    if re.search(r"objequalsVia_|equalsBy_|objequalsBy_|equalsDyn_", sink):
        return "DONE", "the Object argument of equals (jsgo Equals(other any))"
    if re.search(r"objstr_|defaultToString_|tostr", sink) or "ToStringHelper" in cat or "strBuilder_" in cat or "Joiner" in cat or "Output.append" in sink or re.search(r"Appendable|append_CharSequence|appendTo|appendWith|appendFirstField", sink):
        return "REASON", "Java API takes Object or CharSequence (toString helpers, StringBuilder.append(Object), Joiner, Appendable)"
    if "AssertionError" in sink or "MalformedTypedAstException" in sink or "notEqual" in sink or "reportExpectedError" in sink:
        return "REASON", "Java signature takes Object (exception or message argument)"
    if cat == "local variable": return "REASON", "Double/Integer local used as a nullable number (jsgo: (T, bool) results by hand)"
    if cat == "generated field": 
        if "refsForNode" in sink: return "REASON", "Object field holding two types (GlobalNamespace.Name.refsForNode)"
        return "RULE R5", "Object field holding one boxed primitive (tri-state Boolean/Integer)"
    if re.search(r"Node_Prop_MAP|ImmutableMap_Builder\[\*Node_Prop", calls) or ("g_0casts.go" == d["file"] and "func(" in src) or "func() any { return f_().box_() }" in src:
        return "RULE T7", "converting closure of a func value with a type argument any (func-typed type arguments)"
    if re.search(r"setItem", sink): return "RULE R4", "generic program class with an erased type parameter (SourcePosition<T>.setItem)"
    if "clone_impl" in d["fn"] or "TypeProperty" in d["fn"]: return "DONE", "JSDocInfo.Property value methods take Object (jsgo EqualValues(left any, right any), rhino/js_doc_info.go:259)"
    if re.search(r"Optional_base|ImmutableList_of_Object|ImmutableMap_of_Object|Range_closed|TreeMap|EnumSet|EnumMap|ErasedImmutable|ImmutableSorted|ImmutableCollection_Builder|ImmutableMap_Builder|Ordering|LoadingCache|NavigableSet|ConcurrentMap|ListeningExecutor|CharMatcher|ImmutableSortedSet_Builder", sink + " " + cat + " " + calls):
        return "RULE T2", "argument of an erased library type's method (Guava immutables, Optional, Ordering, TreeMap, EnumSet...)"
    if cat in ("generated interface method parameter", "runtime method parameter (MapT)", "runtime method parameter (TMap)") and re.search(r"contains|indexOf|containsKey|remove|get_t|remove_t|\.get#", sink):
        return "RULE T3", "Object parameter of a typed collection method (Map.get/containsKey/remove, Collection.contains, List.indexOf take Object in Java)"
    if cat == "generated interface method parameter" or "TMap.Put" in sink:
        return "RULE T3", "erased element argument of a collection method (Map.put, List.add, typed map Put of an erased value)"
    if re.search(r"splitToListT_|Map_computeIfAbsent|MoreObjects_firstNonNull|Object_clone|Map_put_s|Map_putIfAbsent_s|Map_replace|Collections_singletonList|Integer_parseInt_CharSequence|Ascii_equalsIgnoreCase|TextFormat_merge", sink):
        return "RULE T1", "argument of an erased runtime library function (" + sink.split("#")[0] + ")"
    if re.search(r"elemsAs_|elemsBy_|foreach_", sink): return "RULE T6", "for-each over a typed value boxed to Object"
    if re.search(r"Object_getClass|System_identityHashCode|closeResource_", sink): return "DONE", "Object API of the JDK (getClass, identityHashCode, AutoCloseable close)"
    if cat == "runtime method parameter (BigInteger)" or cat == "runtime method parameter (ComparisonChain)" or cat == "runtime method parameter (Enum)":
        return "RULE T4", "Comparable/compareTo argument (BigInteger.compareTo, ComparisonChain.compare, Enum.compareTo)"
    if cat == "result of a func literal":
        if re.search(r"ToStringHelper|String_format|ErrorReporter|objstr_|strBuilder|Appendable", calls):
            return "REASON", "Java API takes Object (toString / format argument computed in a lambda)"
        if re.search(r"Iterables_|Lists_|Map_|CacheLoader|Arrays_setAll|thenComparing|computeIfAbsent|single_|Supplier_base|getPropWithDefault|new_ColorSerializer|submit|Source_Lazy_memoize", calls):
            return "RULE T1", "result of a lambda given to an erased runtime library function"
        if "doGetNumberValue" in d["fn"] or "parseRepetition" in d["fn"]:
            return "REASON", "Double/Integer result used as a nullable number (jsgo: (T, bool) results by hand)"
        if "createFromKey" in d["fn"] or "getId" in d["fn"]:
            return "REASON", "Object result holding two types in Java"
        if "packPropertyValues" in d["fn"]: return "DONE", "JSDocInfo property values packed as Object (jsgo propertyValues any)"
    if cat == "generated result":
        if re.search(r"next|computeNext", sink): return "RULE T4", "bridge of a program class implementing Iterator<T> (next returns Object)"
        if re.search(r"clone|load", sink): return "RULE T4", "bridge of a program class implementing a library interface (clone, CacheLoader.load)"
        return "REASON", "Object result in Java (" + sink + ")"
    if cat.startswith("argument of a func"): return "RULE T7", "argument of a func value with an erased type argument"
    if d["file"] == "g_0casts.go" and cat == "result of a func literal":
        return "RULE T7", "converting closure of a func value with a type argument any (func-typed type arguments)"
    if "Comparator_comparing" in src: return "RULE T1", "key of Comparator.comparing typed any (the runtime's comparing erases U extends Comparable)"
    if sink == "append#1" and "NodeTraversal" in d["file"]: return "REASON", "Deque<Object> holding Nodes and scopes (NodeTraversal's scope stack)"
    if "connectDiGraphNode" in sink: return "REASON", "Java LinkedDirectedGraph<ColorGraphNode, Object>: Object edge values"
    if "Source_Lazy_1" in sink: return "RULE T4", "bridge of a class implementing a library interface (Source.Lazy, Supplier<String>)"
    return "UNSPLIT", cat + " :: " + sink

def ex(d, i):
    p = d["extra"].split(" | "); return p[i] if len(p) > i else ""

def r0(d):
    m = d["matched"]
    if "BitSet" in m: return "DONE", "BitSet (jsgo jutil.BitSet, jutil/bitset.go:13)"
    if re.search(r"LinkedList|ArrayDeque|EnumMap|TreeMap|IdentityHashMap", m):
        return "RULE T2", "erased library class with no typed twin used here (Deque, EnumMap, TreeMap, IdentityHashMap)"
    return "UNSPLIT", "ArrayList / LinkedHashMap / LinkedHashSet wrapped at once by listAs_/mapAs_ or copied (colls left them; per-site reason not read)"

def r1(d): return "UNSPLIT", "typed TList/TMap/TSet constructor (" + re.sub(r"\(.*", "", d["matched"]) + ")"
def r2(d): return "RULE T2", "erased Guava immutable or builder (" + re.sub(r"(_Object)+", "_Object..", d["matched"].rstrip("(")) + ")"
def r3(d): return "DONE", "make([]T) (jsgo 732)"

def r5(d):
    if d["matched"].startswith(".each"):
        return "UNSPLIT", ".each() cursor over a typed list or set (native keeps a cursor where the loop may change the collection; not split per site)"
    nxt = d["src"]
    if re.search(r"get\w+List\(\)", nxt): return "RULE T5", "for-each over a protobuf list accessor (erased List)"
    if "foreach_(obj_(" in nxt: return "RULE T6", "for-each over a typed value boxed to Object (program Iterable<T>, EnumSet)"
    if re.search(r"getInEdges|getOutEdges", nxt): return "REASON", "Java wildcard List<? extends DiGraphEdge<N, E>> (no variance in Go; jsgo's graph is concrete, by hand)"
    if re.search(r"Lists_reverse|split_s|Io_listFiles", nxt): return "RULE T1", "for-each over an erased runtime library function's result"
    if "currentClassStack" in nxt: return "RULE T2", "for-each over an erased Deque"
    return "UNSPLIT", nxt.strip()

def r6(d):
    if "this.hasNext" in d["src"]: return "REASON", "hasNext() inside a program class implementing Iterator (Java's protocol; jsgo has 3 iterator types)"
    return "REASON", "explicit Iterator walk written by hand in Java (iters converts 0 on this tree; a range or index loop instead is a rewrite of the walk; jsgo has 16 HasNext)"

def r12(d):
    if d["sub"].startswith("declaration"): return "not a site", "is_X helper declaration"
    return "DONE", "instanceof (jsgo typeCheck_isFunctionType one-line helpers, type_check_visit.go:75-80)"

def r13(d):
    s = d["sub"]
    if s.startswith("virtual dispatch"): return "DONE", "dispatcher (exact-class test or single implementation; jsgo self_ JSType, jstype_named_type.go:128)"
    if s.startswith("other use of this.self_"): return "DONE", "other this.self_.(T)"
    if s.startswith("body of"): return "not a site", "body of a cast/instanceof helper"
    if s.startswith("instanceof as comma-ok"): return "DONE", "instanceof as comma-ok assertion (jsgo 284 comma-ok assertions)"
    if s.startswith("Java cast folded"):
        src = s.split(" of: ", 1)[1]
        src = re.sub(r"^(local (Object \(any\)|a Java interface) initialised from: )+", "", src)
        if src.startswith("self()") or "GUARDED" in d["extra"]: return "DONE", "cast folded to v.(T): self() of a typed value or guarded"
        if "tconv_" in d["detail"]: return "not a site", "inside the unbox function of a tconv_ descriptor"
        if re.search(r"getDeclarationScope|SemanticReverseAbstractInterpreter_unwrap|new_ReferenceCollector", d["detail"] + d["sub"]) or "Java interface of a generated function" in s or "local a Java interface initialised" in s:
            return "DONE", "Java downcast of an interface value to its class"
        if "Node.getProp" in d["detail"]: return "DONE", "Node property bag read"
        if "equals" in d["detail"]: return "DONE", "equals(Object) parameter"
        if re.search(r"isDefault|LogsGson_write", d["detail"]): return "REASON", "Object slot holding values of two or more types in Java"
        if re.search(r"toBuilder", d["detail"]): return "RULE T5", "protobuf builder returning an erased value"
        if "lgBox_" in d["detail"] and re.search(r"Imm_", d["calls"]): return "RULE T2", "erased Guava immutable twin read back (lgBox_(..).(*ErasedImmutable..))"
        if re.search(r"Optional_base", d["detail"]): return "RULE T2", "value of an erased library type (Optional_base)"
        if re.search(r"Map_Entry", d["detail"]): return "RULE T2", "entry of an erased library map"
        return "RULE T1", "erased runtime library function or converting closure (" + d["detail"][:40] + ")"
    return "UNSPLIT", s[:60]

def r14(d):
    if "helper" in d["sub"]: return "not a site", "type switch in a to_X / gcast_X helper"
    return "DONE", "type switch (jsgo 78; visitors as type switches, jstype_template_type_replacer.go:211)"

LAMBDA_DONE = ("CompilerPass", "Source_Transformer", "ModuleLoader_ModuleResolverFactory", "Matcher", "SourceMap_LocationMapping", "ErrorHandler", "CheckConformance_Precondition", "CompilerInputProvider")
def lam(d):
    fam = re.sub(r"_t?fn(_.*)?(\[.*)?$", "", ex(d, 0)) if d["row"] == "18" else ex(d, 0).replace("_fn", "")
    fam = fam.split(" ")[0]
    if fam in LAMBDA_DONE: return "DONE", "lambda object of an interface a class implements too (jsgo: func type with a method or a small struct)"
    if fam in ("NodeUtil_Visitor", "Runnable"): return "RULE F1", "lambda object where jsgo passes a plain func (NodeUtil_VisitPreOrder func(rhino.Node); deferAction(fn func()))"
    if fam == "ThreadFactory": return "REASON", "executor plumbing (jsgo wrote goroutine pools by hand)"
    return "UNSPLIT", "lambda object " + fam + " (Iterable lambda given to iterableAs_; jsgo form not looked up)"

def r19(d):
    if d["sub"].startswith("NOT CODE"): return "not a site", "string literal"
    if d["matched"] == "Optional": return "DONE", "java.util.Optional as the typed value Optional[T] (jsgo: (T, bool) results, nilable pointers)"
    return "RULE T2", "erased Guava Optional (Optional_base_*)"

def r20(d): return "DONE", "try/catch emulation (recover at boundaries, catch_ of the recovered value; jsgo 58 recover())"
def r21(d): return "DONE", "panic of a Java exception (jsgo panic(jutil.NewX(..)) 1,724)"
def r22(d): return "DONE", "defer (finally, try-with-resources, catch)"

def r23(d):
    m, a0 = d["matched"], ex(d, 0)
    if m.startswith("Objects_equals") and re.search(r"^arg0 (Map|List|Set)\b", a0): return "RULE R7", "Objects.equals of two erased views of typed collections"
    if m.startswith("Objects_requireNonNull"): return "DONE", "requireNonNull used for its value (jsgo jutil.CheckNotNull[T])"
    return "DONE", "equals/hashCode protocol on Object values (jsgo ObjectsEquals objects.go:34, 70 Equals methods)"

def r24(d): return "DONE", "UTF-16 String helper (jsgo jutil/jstring.go StringLength, StringCharAt...)"
def r25(d):
    if d["sub"].startswith("NOT CODE"): return "not a site", "name table"
    if d["sub"] == "type name": return "not a site", "StringBuilder_as_Appendable helper declaration"
    return "RULE R7", "Appendable(new_StringBuilder()) (manual step 303 on the lead line)"
def r26(d): return "RULE R7", "Node_Prop as an object enum: enums refuses (the runtime names Node_Prop_CONSTANT_VAR_FLAGS); jsgo type Node_Prop int32 (rhino/node_props.go:49)"
def r28(d):
    if d["file"] == "g_TemplatizedType.go": return "RULE E-nnchecks", "the existing tool nnchecks makes it an inline check on this tree"
    return "DONE", "Preconditions.checkNotNull (jsgo jutil.CheckNotNull, preconditions.go:236)"
def r29(d):
    if d["sub"].startswith("NOT CODE"): return "not a site", "name table or comment"
    m = d["matched"]
    if m in ("CompilerExecutor", "Thread"): return "DONE", "CompilerExecutor / one-thread Thread (jsgo compiler_pipeline.go:690-701, jutil/logger.go:370-378)"
    return "REASON", "ExecutorService / Future / submit (jsgo wrote goroutine pools by hand, prebuild_dependency_info_parallel.go:58-63)"
def r30(d): return "not a site", "string literal (reflect.) or a local named unsafe"

ROWS = {0: r0, 1: r1, 2: r2, 3: r3, 5: r5, 6: r6, 10: castlike, 11: castlike, 12: r12, 13: r13, 14: r14, 15: boxing, 16: castlike, 17: lam, 18: lam, 19: r19, 20: r20, 21: r21, 22: r22, 23: r23, 24: r24, 25: r25, 26: r26, 27: r26, 28: r28, 29: r29, 30: r30, 31: r30}

ANYS = [l.rstrip("\n").split("\t") for l in open("anyslots.tsv")]
R5F = ("browserFeaturesetYear", "parseInlineSourceMaps_", "polymerVersion", "templateCheckResult", "considerExterns", "isUsingStructuralTyping", "cap_instance")
def anyslot(a):
    f, ln, kind, name, m, src = a
    var = m.split()[0]
    if kind == "field":
        if src.startswith("if any("): return "not a site", "`if any(this) == x` (regex match)"
        if var in R5F: return "RULE R5", "Object field holding one boxed primitive or one type (tri-state Boolean/Integer, Source.Lazy's String)"
        if var in ("propertyValues", "objectValue"): return "DONE", "property bag (jsgo js_doc_info.go:75, node_props.go:382 are any)"
        if var == "item": return "RULE R4", "SourcePosition<T>.item: an erased type parameter"
        return "REASON", "Object field holding two or more types (" + var + ")"
    if kind == "other":
        if "valueT_" in src: return "DONE", "typed reader of the JSDocInfo property bag (jsgo jsdocGet[T])"
        if "enumList" in src: return "RULE R5", "command-line option setter of a nullable boxed flag"
        return "not a site", "type parameter's helper parameter (to_X of a generic class)"
    if kind == "funclit":
        if f == "g_0casts.go":
            if "unbox:" in src or re.search(r"tconv_|unbox", src): return "not a site", "parameter of a tconv_ descriptor's unbox function"
            return "RULE T7", "converting closure of a func-typed conversion (g_0casts.go)"
        if "map_EntryAs_" in line(f, int(ln) + 1) or "map_EntryAs_" in src:
            return "REASON", "typed sort of typed entries (REFLIP6 'typed sorts': wrappers break K(box(x)) == x; a rule was tried and dropped)"
        if f == "g_CommandLineRunner.go": return "RULE R5", "command-line option setter of a nullable boxed flag (Consumer[any] for Boolean/Integer)"
        if f == "g_Node.go": return "RULE T7", "func type as a type argument of a typed container (Node.PROP_MAP_FOR_EQUALITY)"
        if re.search(r"Comparator\[any\]|newTSortedSetC_|List_sort|Collections_sort|Map_forEach|Iterable_forEach|Iterables_|Lists_|Map_computeIfAbsent|Map_replaceAll|CacheLoader|BiConsumer\[any|Consumer\[any|Predicate_base\[any|Function_base\[any|Function\[any", src):
            return "RULE T1", "parameter of a converting closure given to an erased runtime library function"
        if re.search(r"ImmutableSorted|Optional_base|Ordering", src): return "RULE T2", "parameter of a converting closure given to an erased Guava type"
        if src.startswith("return func(p0 any"): return "RULE T1", "parameter of a converting closure (typed function value given to an erased runtime function)"
        if "tconv_[any]" in src: return "not a site", "parameter of a tconv_ descriptor's unbox function"
        if "FixedPointGraphTraversal" in src: return "REASON", "Object edge values of LinkedDirectedGraph<ColorGraphNode, Object>"
        if "computeT_" in src: return "RULE R5", "Integer value of a typed map held as any (nullable boxed Integer)"
        return "UNSPLIT", "func literal parameter: " + src[:80]
    # methods and functions
    if name in ("equals", "equals_impl", "Object_equals"): return "DONE", "equals(Object) parameter (jsgo Equals(other any), var.go:568)"
    if re.match(r"(equalValues|clone|getTypeExpressions_Object)(_impl)?$", name) or name in ("isDefault",) and "JSDocInfo" in f:
        return "DONE", "JSDocInfo.Property value methods take Object (jsgo EqualValues(left any, right any), rhino/js_doc_info.go:259)"
    if name in ("setItem", "setItem_impl"): return "RULE R4", "SourcePosition<T>.setItem: an erased type parameter"
    if name in ("compareTo", "compare"): return "RULE T4", "bridge of Comparable/Comparator (erased callers: the runtime's sorts)"
    if name in ("apply", "contains", "add", "load", "put", "containsKey", "containsValue"): return "RULE T4", "bridge of a class implementing a library interface (" + name + ")"
    if name in ("valueT_",): return "DONE", "typed reader of the JSDocInfo property bag"
    if re.match(r"(putPropEntry|setProp|populatePropEntry|populateProp|payload_set_|jstypeOrColor_set_|new_Node_ObjectPropListItem|Node_payloadClass|Node_of_)$", name):
        return "DONE", "Node / JSDocInfo property bag (jsgo PutProp(prop, value any))"
    if name in ("setBrowserFeaturesetYear", "setPolymerVersion", "CommandLineRunner_Flags_addMultiFlagValue"): return "RULE R5", "setter of a tri-state boxed flag"
    if re.match(r"(appendTo|appendTo_impl|append_CharSequence\w*|SourceCodeEscapers_appendWithJavascriptEscaper|SourceMapGeneratorV3_appendFirstField)$", name):
        return "REASON", "CharSequence or Object in Java (charseq leaves it: through interface Appendable / not a boxed String)"
    if re.match(r"(logObject|logJsonObject)(_impl)?$|LogsGson_|new_AssertionError_Object|MalformedTypedAstException|ChangeVerifier_EqualsResult_notEqual|DotFormatter_formatNodeName|JSCompObjects_identical|HamtPMap_hash|J2clEqualitySameRewriterPass_isSafeNumber|newNumberNode|favors|traverseEdge|addSupertypeEdges|new_Property|new_ClosureBundler|init_ClosureBundler|CmdLineParser_Option|cfgEdge_", name):
        return "REASON", "Object in Java (" + name + ")"
    if name == "new_Source_Lazy_1": return "RULE R5", "Source.Lazy's captured String held as Object"
    if f == "g_zstubs.go": return "not a site", "j2go stub of a library class (g_zstubs.go: members not yet in the runtime)"
    if name == "init_Node_ObjectPropListItem": return "DONE", "Node / JSDocInfo property bag (jsgo PutProp(prop, value any))"
    if name == "addEvent": return "RULE T2", "erased library map (MapOf alias) of Timeline"
    if re.match(r"init_MalformedTypedAstException|new_MalformedTypedAstException|init_AssertionError_Object|reportExpectedError|addExtension|setReflectedObject|init_Property|isReasonableDoubleValue|join_1|getNodeRootFromScopeObj|addNewEntry", name):
        return "REASON", "Object in Java (" + name + ")"
    return "UNSPLIT", kind + " " + name

def main():
    out = open("verdicts.tsv", "w")
    tab = collections.defaultdict(collections.Counter)
    vrow = int(sys.argv[2]) if len(sys.argv) > 2 and sys.argv[1] == "-v" else None
    for d in S:
        r = int(d["row"])
        if r not in ROWS: continue
        v, lab = ROWS[r](d)
        tab[r][(v, lab)] += 1
        out.write("\t".join([d["row"], d["file"], d["line"], v, lab]) + "\n")
        if vrow == r:
            print("%s:%s\t%s\t%s\t%s\t%s" % (d["file"], d["line"], v, lab, d["calls"][:90], d["src"].strip()[:120]))
    for a in ANYS:
        v, lab = anyslot(a)
        tab[100][(v, lab)] += 1
        out.write("\t".join(["100", a[0], a[1], v, lab]) + "\n")
        if vrow == 100: print("%s:%s\t%s\t%s\t%s" % (a[0], a[1], v, lab, a[5][:120]))
    if vrow is not None: return
    for r in sorted(tab):
        c = tab[r]
        tot = collections.Counter()
        for (v, lab), n in c.items(): tot[v] += n
        print("\n### row %d: %d  (%s)" % (r, sum(c.values()), ", ".join("%s %d" % kv for kv in sorted(tot.items()))))
        for (v, lab), n in sorted(c.items(), key=lambda x: (x[0][0], -x[1])):
            print("| %s | %s | %d |" % (v, lab, n))

main()
