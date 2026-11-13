package closure

// Tests of the protobuf runtime against protobuf-java. The fixtures under
// testdata/proto were written by testdata/proto/Gen.java.txt (run against the
// real generated classes); the tests rebuild, re-parse and re-print the same
// messages in Go and expect identical bytes and text.

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pbFixture_(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "proto", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func pbLines_(t *testing.T, name string) [][]string {
	var out [][]string
	for _, line := range strings.Split(strings.TrimRight(string(pbFixture_(t, name)), "\n"), "\n") {
		out = append(out, strings.Split(line, "\t"))
	}
	return out
}

// pbThrown_ runs f and returns the message of the exception it throws.
func pbThrown_(f func()) (class string, msg string) {
	defer func() {
		if r := recover(); r != nil {
			ex := r
			if _, ok := r.(interface{ as_Throwable() *Throwable }); !ok {
				ex = catch_(r)
			}
			class = Object_getClass(ex).getName()
			msg = ex.(interface{ getMessage() string }).getMessage()
		}
	}()
	f()
	return "", "OK"
}

func pbParseDeep_(full string, data []byte) any {
	in := (&ByteString{b_: data}).newCodedInput()
	in.setRecursionLimit(math.MaxInt32)
	return pbParseCoded_(pbDescs_[full], in)
}

func pbTestLeaf_() *AstNode {
	return AstNode_newBuilder().setKind(NodeKind_IDENTIFIER).setStringValuePointer(7).setRelativeLine(-3).
		setRelativeColumn(200).setOriginalNamePointer(-1).setBooleanProperties(-5).setType(0).setSourceFile(3).build().(*AstNode)
}

func pbTestRoot_() *AstNode {
	tmpl := AstNode_newBuilder().setKind(NodeKind_TEMPLATELIT_STRING).
		setTemplateStringValue(TemplateStringValue_newBuilder().setRawStringPointer(4).setCookedStringPointer(-1).build().(*TemplateStringValue)).build().(*AstNode)
	num := AstNode_newBuilder().setKind(NodeKind_NUMBER_LITERAL).setDoubleValue(0).build().(*AstNode)
	num2 := AstNode_newBuilder().setKind(NodeKind_NUMBER_LITERAL).setDoubleValue(-1.25e300).setRelativeLine(1 << 30).build().(*AstNode)
	nan := AstNode_newBuilder().setKind(NodeKind_NUMBER_LITERAL).setDoubleValue(math.NaN()).build().(*AstNode)
	inf := AstNode_newBuilder().setKind(NodeKind_NUMBER_LITERAL).setDoubleValue(math.Inf(-1)).build().(*AstNode)
	jsdoc := AstNode_newBuilder().setKindValue(9999).setJsdoc(OptimizationJsdoc_newBuilder().
		setLicenseTextPointer(5).addKind(JsdocTag_JSDOC_CONST).addKind(JsdocTag_JSDOC_NO_INLINE).addKindValue(4242).setMeaningPointer(9).build().(*OptimizationJsdoc)).build().(*AstNode)
	return AstNode_newBuilder().setKind(NodeKind_SOURCE_FILE).addChild(pbTestLeaf_()).addChild(tmpl).addChild(num).addChild(num2).
		addChild(nan).addChild(inf).addChild(jsdoc).addChild(AstNode_getDefaultInstance()).setStringValuePointer(0).build().(*AstNode)
}

func pbInts_(v ...int32) Iterable {
	a := make([]any, len(v))
	for i, x := range v {
		a[i] = x
	}
	return listOf_(a)
}

func pbStrs_(v ...string) Iterable {
	a := make([]any, len(v))
	for i, x := range v {
		a[i] = x
	}
	return listOf_(a)
}

func pbTestTypedAst_() *TypedAst {
	root, leaf := pbTestRoot_(), pbTestLeaf_()
	tp := TypePool_newBuilder()
	obj := ObjectTypeProto_newBuilder().setIsInvalidating(true).
		setUuid(ByteString_copyFrom([]int8{0, 1, 2, -1, -128, '"', '\\', '\n'})).
		addAllInstanceType(pbInts_(1, 2, 300, -1)).addAllPrototype(pbInts_(70000)).addAllOwnProperty(pbInts_(0, 127, 128)).
		setClosureAssert(true)
	obj.getDebugInfoBuilder().addTypenamePointer(3)
	tp.addType(TypeProto_newBuilder().setObject(obj.build().(*ObjectTypeProto)).build().(*TypeProto))
	u := TypeProto_newBuilder()
	u.getUnionBuilder().addAllUnionMember(pbInts_(1, 2))
	tp.addType(u.build().(*TypeProto))
	tp.addType(TypeProto_newBuilder().setUnion(UnionTypeProto_getDefaultInstance()).build().(*TypeProto))
	tp.addType(TypeProto_getDefaultInstance())
	tp.addDisambiguationEdges(SubtypingEdge_newBuilder().setSubtype(1).setSupertype(2))
	tp.addDisambiguationEdgesBuilder().setSubtype(5)
	tp.getDebugInfoBuilder().addMismatchBuilder().setSourceRef("a.js:1:2").addAllInvolvedColor(pbInts_(4, 5))

	sfp := SourceFilePool_newBuilder().addAllSourceFile(listOf_([]any{
		SourceFileProto_newBuilder().setFilename("a.js").setPreloadedContents("var x = 'h\u00e9llo \u4e16\u754c \U0001F600';\n").
			setSourceKind(SourceFileProto_SourceKind_CODE).setNumLinesPlusOne(2).setNumBytesPlusOne(40).build(),
		SourceFileProto_newBuilder().setFilename("b.js").setFileOnDisk(SourceFileProto_FileOnDisk_newBuilder().setActualPath("/x/b.js").setCharset("UTF-8")).
			setSourceKind(SourceFileProto_SourceKind_EXTERN).build(),
		SourceFileProto_newBuilder().setFilename("c.js").setZipEntry(SourceFileProto_ZipEntryOnDisk_newBuilder().setZipPath("z.zip").setEntryName("c.js").build().(*SourceFileProto_ZipEntryOnDisk)).
			setSourceKindValue(77).setIsClosureUnawareCode(true).build(),
		SourceFileProto_newBuilder().setFilename("d.js").setStubFile(false).build(),
		SourceFileProto_newBuilder().setPreloadedContents("").build(),
	})).build().(*SourceFilePool)

	return TypedAst_newBuilder().setTypePool(tp.build().(*TypePool)).
		setStringPool(StringPoolProto_newBuilder().setMaxLength(12).addStrings(ByteString_EMPTY).addStrings(ByteString_copyFromUtf8("hello")).
			addStrings(ByteString_copyFrom([]int8{-19, -96, -128})).build().(*StringPoolProto)).
		setSourceFilePool(sfp).
		addCodeAst(LazyAst_newBuilder().setScript(root.toByteString()).setSourceFile(1).setSourceMappingUrl("a.js.map").build().(*LazyAst)).
		addCodeAst(LazyAst_newBuilder().setScript(leaf.toByteString()).setSourceFile(2).build().(*LazyAst)).
		addExternAst(LazyAst_getDefaultInstance()).
		setExternsSummary(ExternsSummary_newBuilder().addPropNamePtr(1).addPropNamePtr(1000000).addPropNamePtr(-7)).
		addAllRuntimeLibraryToInject(pbStrs_("es6/util", "base")).
		build().(*TypedAst)
}

func pbTestConformance_() *ConformanceConfig {
	r1 := Requirement_newBuilder().setErrorMessage("eval is \"bad\"\n\t\\ caf\u00e9 \u0001\u007f '").
		addAllWhitelist(pbStrs_("a/")).addAllWhitelistRegexp(pbStrs_(".*test.*")).addAllAllowlist(pbStrs_("b/")).addAllAllowlistRegexp(pbStrs_("x+")).
		addAllAllowlistEntry(listOf_([]any{RequirementScopeEntry_newBuilder().setReason(RequirementScopeEntry_Reason_LEGACY).addPrefix("p/").addRegexp("r").
			setExplanation("why").addComment("c1").addComment("c2").setAutomaticallyPrune(false).build()})).
		addAllWhitelistEntry(listOf_([]any{RequirementScopeEntry_getDefaultInstance()})).
		addAllOnlyApplyTo(pbStrs_("src/")).addAllOnlyApplyToRegexp(pbStrs_("re")).
		setType(Requirement_Type_BANNED_NAME).addAllValue(pbStrs_("eval", "")).
		setAllowExtendingValue(true).setTypeMatchingStrategy(Requirement_TypeMatchingStrategy_EXACT).
		setJavaClass("com.Foo").setRuleId("closure:eval").setReportLooseTypeViolations(false).
		setSeverity(Requirement_Severity_ERROR).addAllConfigFile(pbStrs_("conf.textproto")).build().(*Requirement)
	r2 := Requirement_newBuilder().setExtends("closure:eval").setType(Requirement_Type_CUSTOM).
		setSeverity(Requirement_Severity_UNSPECIFIED).setTypeMatchingStrategy(Requirement_TypeMatchingStrategy_LOOSE).setReportLooseTypeViolations(true).build().(*Requirement)
	return ConformanceConfig_newBuilder().addRequirement(r1).addRequirement(r2).addRequirement(Requirement_getDefaultInstance()).
		setLibraryLevelNonAllowlistedConformanceViolationsBehavior(ConformanceConfig_LibraryLevelNonAllowlistedConformanceViolationsBehavior_RECORD_ONLY).
		build().(*ConformanceConfig)
}

func pbTestState_() *JSCompilerStateProto {
	return JSCompilerStateProto_newBuilder().
		setTypeCheckingHasRun(true).setHasRegExpGlobalReferences(false).setLifeCycleStage(LifeCycleStageProto_forNumber(1)).
		addExterns("externs.js").addExterns("more.js").setUniqueNameId(42).addAllExportedNames(pbStrs_("x")).setIdGeneratorMap("").
		addChunks(ChunkProto_newBuilder().setName("main").addInputIds("a").addInputIds("b").addDependencies(0).addDependencies(3).build().(*ChunkProto)).
		addAllowableFeatures(FeatureProto_forNumber(1)).addAllowableFeatures(FeatureProto_forNumber(0)).addAllowableFeaturesValue(555).
		addAllUniqueIdSupplier(listOf_([]any{UniqueIdProto_newBuilder().setHash(-123).setCounter(4).build()})).
		setAccessorSummary(AccessorSummaryProto_newBuilder().setAssumeAlwaysGetterAndSetter(true).
			addAccessors(AccessorSummaryEntryProto_newBuilder().setName("p").setKind(PropertyAccessKindProto_forNumber(2)).build().(*AccessorSummaryEntryProto)).build().(*AccessorSummaryProto)).
		addAllStringMap(listOf_([]any{VariableMapEntryProto_newBuilder().setOriginalName("o").setNewName("n").build()})).setHasStringMap(true).
		setIdGeneratorCurrentId(3).setLastInjectedLibraryIndexInFirstScript(-1).addAllInjectedLibraries(pbStrs_("base")).
		build().(*JSCompilerStateProto)
}

// TestProtoBuild builds the fixtures' messages through the generated builders.
func TestProtoBuild(t *testing.T) {
	check := func(name string, m pbM_) {
		t.Helper()
		if got, want := m.pbm_().bytes_(), pbFixture_(t, name+".bin"); !bytes.Equal(got, want) {
			t.Errorf("%s: bytes differ:\n got %x\nwant %x\n%s", name, got, want, m.pbm_().toString())
		}
		if got, want := m.pbm_().toString(), string(pbFixture_(t, name+".txt")); got != want {
			t.Errorf("%s: text differs:\n got %s\nwant %s", name, got, want)
		}
	}
	check("astnode", pbTestRoot_())
	check("astnode_empty", AstNode_getDefaultInstance())
	ta := pbTestTypedAst_()
	check("typedast", ta)
	check("typedast_list", TypedAst_List_newBuilder().addTypedAsts(ta).addTypedAsts(TypedAst_getDefaultInstance()).addTypedAsts(ta).build().(*TypedAst_List))
	check("conformance", pbTestConformance_())
	check("state", pbTestState_())
	check("mapping", Mapping_OriginalMapping_newBuilder().setOriginalFile("f.js").setLineNumber(0).setColumnPosition(-1).
		setPrecision(Mapping_OriginalMapping_Precision_UNKNOWN_PRECISION).build().(*Mapping_OriginalMapping))
	check("mapping_empty", Mapping_OriginalMapping_getDefaultInstance())
	stats := InstrumentationPointStats_newBuilder().setTimesExecuted(1 << 40).setPointPresence(InstrumentationPointStats_Presence_PRESENT).
		setPoint(InstrumentationPoint_newBuilder().setType(InstrumentationPoint_Type_BRANCH_DEFAULT).setFileName("f.js").setFunctionName("fn").
			setLineNumber(1).setColumnNumber(0).build().(*InstrumentationPoint)).build()
	check("profile", ReportProfile_newBuilder().addAllFileProfile(listOf_([]any{FileProfile_newBuilder().setFileName("f.js").
		addAllInstrumentationPointsStats(listOf_([]any{stats, InstrumentationPointStats_getDefaultInstance()})).build()})).build().(*ReportProfile))
}

// TestProtoCodedBytes writes and reads a size-prefixed message followed by raw bytes, the way
// Compiler.saveState and restoreState do.
func TestProtoCodedBytes(t *testing.T) {
	msg := pbTestLeaf_()
	size := msg.getSerializedSize()
	tail := []int8{1, -2, 3}
	if n := len(msg.toByteArray()); int32(n) != size {
		t.Fatalf("toByteArray has %d bytes, getSerializedSize says %d", n, size)
	}
	buf := make([]int8, CodedOutputStream_computeUInt32SizeNoTag(size)+size+int32(len(tail)))
	out := CodedOutputStream_newInstance(buf)
	out.writeUInt32NoTag(size)
	msg.writeTo(out)
	out.writeRawBytes(tail)
	out.flush()
	if class, _ := pbThrown_(func() { out.writeRawBytes(tail) }); class != "java.io.IOException" {
		t.Errorf("writing past the end: %s", class)
	}
	in := CodedInputStream_newInstance(buf)
	limit := in.pushLimit(in.readRawVarint32())
	got := AstNode_parseFrom_CodedInputStream(in)
	in.popLimit(limit)
	if !msg.equals(got) {
		t.Errorf("read back %v", got)
	}
	rest := Arrays_copyOfRange(buf, in.getTotalBytesRead(), int32(len(buf)))
	if len(rest) != 3 || rest[0] != 1 || rest[1] != -2 || rest[2] != 3 || in.isAtEnd() {
		t.Errorf("after the message: %v, at end %v", rest, in.isAtEnd())
	}
	in.pushLimit(2)
	if class, _ := pbThrown_(func() { in.pushLimit(3) }); class != "com.google.protobuf.InvalidProtocolBufferException" {
		t.Errorf("limit past the current limit: %s", class)
	}
}
