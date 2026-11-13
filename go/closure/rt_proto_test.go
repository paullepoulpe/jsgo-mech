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

// pbTestIn_ is a java.io.InputStream over a byte slice that hands out at most
// seven bytes per read.
type pbTestIn_ struct {
	InputStream
	data []byte
}

func newPbTestIn_(data []byte) *pbTestIn_ {
	in := &pbTestIn_{data: data}
	in.self_ = in
	return in
}

func (this *pbTestIn_) read_byteArr_int_int(b []int8, off int32, n int32) int32 {
	if len(this.data) == 0 {
		return -1
	}
	if n > 7 {
		n = 7
	}
	if int(n) > len(this.data) {
		n = int32(len(this.data))
	}
	copy(pbU8_(b[off:off+n]), this.data[:n])
	this.data = this.data[n:]
	return n
}

type pbTestOut_ struct {
	OutputStream
	data []byte
}

func newPbTestOut_() *pbTestOut_ {
	out := &pbTestOut_{}
	out.self_ = out
	return out
}

func (this *pbTestOut_) write_byteArr_int_int(b []int8, off int32, n int32) {
	this.data = append(this.data, pbU8_(b[off:off+n])...)
}

func pbParseDeep_(full string, data []byte) any {
	in := (&ByteString{b_: data}).newCodedInput()
	in.setRecursionLimit(math.MaxInt32)
	return pbParseCoded_(pbDescs_[full], in)
}

func TestProtoRoundTrip(t *testing.T) {
	for _, row := range pbLines_(t, "manifest.tsv") {
		name, full := row[0], row[1]
		d := pbDescs_[full]
		if d == nil {
			t.Fatalf("%s: no descriptor for %s", name, full)
		}
		data := pbFixture_(t, name+".bin")
		msg := pbParseDeep_(full, data)
		m := msg.(pbM_).pbm_()
		if got := m.bytes_(); !bytes.Equal(got, data) {
			t.Errorf("%s: re-serialised bytes differ:\n got %x\nwant %x", name, got, data)
		}
		if int(m.getSerializedSize()) != len(data) {
			t.Errorf("%s: getSerializedSize %d, want %d", name, m.getSerializedSize(), len(data))
		}
		text := string(pbFixture_(t, name+".txt"))
		if got := Object_toString(msg); got != text {
			t.Errorf("%s: toString differs:\n got %q\nwant %q", name, got, text)
		}
		if got := TextFormat_printer().printToString(msg.(MessageOrBuilder)); got != text {
			t.Errorf("%s: printToString differs", name)
		}
		if got, want := TextFormat_shortDebugString(msg.(MessageOrBuilder)), string(pbFixture_(t, name+".short")); got != want {
			t.Errorf("%s: shortDebugString differs:\n got %q\nwant %q", name, got, want)
		}
		var fields []string
		for it := m.me_.(MessageOrBuilder).getAllFields().keySet().iterator(); it.hasNext(); {
			fields = append(fields, it.next().(*Descriptors_FieldDescriptor).getName()+",")
		}
		want := ""
		if len(row) > 2 {
			want = row[2]
		}
		if got := strings.Join(fields, ""); got != want {
			t.Errorf("%s: getAllFields %q, want %q", name, got, want)
		}

		// toBuilder().build() is an equal message with the same bytes.
		again := m.toBuilder().build()
		if !Object_equals(msg, again) || !Object_equals(again, msg) || Object_hashCode(msg) != Object_hashCode(again) {
			t.Errorf("%s: toBuilder().build() is not equal", name)
		}
		if !bytes.Equal(again.(pbM_).pbm_().bytes_(), data) {
			t.Errorf("%s: toBuilder().build() serialises differently", name)
		}

		// Text format round trip (unknown fields are not parseable text).
		if name == "requirement_unknown" || name == "astnode_deep" {
			continue
		}
		b := d.newBuilder_()
		TextFormat_merge(text, b.me_.(Message_Builder))
		parsed := b.build()
		if !bytes.Equal(parsed.(pbM_).pbm_().bytes_(), data) {
			t.Errorf("%s: text format round trip differs:\n%s", name, Object_toString(parsed))
		}
		if !Object_equals(parsed, msg) {
			t.Errorf("%s: text format round trip is not equal", name)
		}
	}
}

func TestProtoUnknownFields(t *testing.T) {
	r := pbParse_(Requirement_desc_, pbFixture_(t, "requirement_unknown.input"), false).(*Requirement)
	if got, want := pbU8_(r.toByteArray()), pbFixture_(t, "requirement_unknown.bin"); !bytes.Equal(got, want) {
		t.Errorf("bytes differ:\n got %x\nwant %x", got, want)
	}
	if got, want := r.toString(), string(pbFixture_(t, "requirement_unknown.txt")); got != want {
		t.Errorf("text differs:\n got %s\nwant %s", got, want)
	}
	if r.hasType() || r.getType() != Requirement_Type_CUSTOM || r.getSeverity() != Requirement_Severity_ERROR {
		t.Errorf("unexpected known fields: %v %v %v", r.hasType(), r.getType(), r.getSeverity())
	}
}

func TestProtoTextParse(t *testing.T) {
	b := ConformanceConfig_newBuilder()
	TextFormat_merge(strobj_(string(pbFixture_(t, "conformance_parse.textproto"))), b)
	c := b.build().(*ConformanceConfig)
	if got, want := pbU8_(c.toByteArray()), pbFixture_(t, "conformance_parse.bin"); !bytes.Equal(got, want) {
		t.Errorf("bytes differ:\n got %x\nwant %x\n%s", got, want, c.toString())
	}
	if got, want := c.toString(), string(pbFixture_(t, "conformance_parse.txt")); got != want {
		t.Errorf("text differs:\n got %s\nwant %s", got, want)
	}
	r := c.getRequirement(0)
	if r.getErrorMessage() != "xAA\u00e9\n" || r.getValueCount() != 2 || r.getValue(1) != "ab" {
		t.Errorf("unexpected requirement: %s", r.toString())
	}
	// A StringBuilder is a CharSequence too.
	b2 := ConformanceConfig_newBuilder()
	TextFormat_getParser().merge(new_StringBuilder_String("requirement { rule_id: 'x' }"), b2)
	if b2.getRequirementCount() != 1 || b2.getRequirement(0).getRuleId() != "x" {
		t.Errorf("merge from StringBuilder: %s", b2.toString())
	}
}

func TestProtoTextParseErrors(t *testing.T) {
	unescape := strings.NewReplacer("\\\\", "\\", "\\n", "\n", "\\t", "\t")
	for _, row := range pbLines_(t, "parse_errors.tsv") {
		input, want := unescape.Replace(row[0]), unescape.Replace(row[1])
		class, got := pbThrown_(func() { TextFormat_merge(input, ConformanceConfig_newBuilder()) })
		if got != want {
			t.Errorf("%q:\n got %q\nwant %q", input, got, want)
		}
		if got != "OK" && class != "com.google.protobuf.TextFormat$ParseException" {
			t.Errorf("%q: threw %s", input, class)
		}
	}
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

func TestProtoAccessors(t *testing.T) {
	root := pbTestRoot_()
	if root.getKind() != NodeKind_SOURCE_FILE || root.getChildCount() != 8 || root.getChildList().size() != 8 {
		t.Fatalf("root: %s", root.toString())
	}
	leaf := root.getChild(0)
	if leaf.getKind() != NodeKind_IDENTIFIER || leaf.getStringValuePointer() != 7 || leaf.getRelativeLine() != -3 ||
		leaf.getBooleanProperties() != -5 || !leaf.hasType() || leaf.getType() != 0 || leaf.getOriginalNamePointer() != -1 {
		t.Errorf("leaf: %s", leaf.toString())
	}
	if leaf.getValueCase() != AstNode_ValueCase_STRING_VALUE_POINTER || root.getValueCase() != AstNode_ValueCase_STRING_VALUE_POINTER {
		t.Errorf("value case: %v %v", leaf.getValueCase(), root.getValueCase())
	}
	if c := root.getChild(2); c.getValueCase() != AstNode_ValueCase_DOUBLE_VALUE || c.getDoubleValue() != 0 || c.getStringValuePointer() != 0 {
		t.Errorf("double child: %v", c.getValueCase())
	}
	empty := root.getChild(7)
	if empty.hasType() || empty.getKind() != NodeKind_NODE_KIND_UNSPECIFIED || empty.getValueCase() != AstNode_ValueCase_VALUE_NOT_SET ||
		empty.getJsdoc() != OptimizationJsdoc_getDefaultInstance() || empty.hasJsdoc() || !empty.equals(AstNode_getDefaultInstance()) {
		t.Errorf("empty child")
	}
	odd := root.getChild(6)
	if odd.getKind() != NodeKind_UNRECOGNIZED || odd.getKindValue() != 9999 || odd.getJsdoc().getKind(2) != JsdocTag_UNRECOGNIZED ||
		odd.getJsdoc().getKindValue(2) != 4242 || odd.getJsdoc().getKindList().get(0) != any(JsdocTag_JSDOC_CONST) {
		t.Errorf("unrecognized enum handling: %s", odd.toString())
	}
	if _, msg := pbThrown_(func() { NodeKind_UNRECOGNIZED.getNumber() }); msg != "Can't get the number of an unknown enum value." {
		t.Errorf("UNRECOGNIZED.getNumber: %q", msg)
	}
	if class, _ := pbThrown_(func() { root.getChild(8) }); class != "java.lang.IndexOutOfBoundsException" {
		t.Errorf("getChild(8) threw %q", class)
	}
	if class, _ := pbThrown_(func() { AstNode_newBuilder().setKind(nil) }); class != "java.lang.NullPointerException" {
		t.Errorf("setKind(null) threw %q", class)
	}
	if class, _ := pbThrown_(func() { Requirement_newBuilder().setRuleId(nullstr_) }); class != "java.lang.NullPointerException" {
		t.Errorf("setRuleId(null) threw %q", class)
	}

	// Builders stay usable after build(), and built messages do not change.
	b := AstNode_newBuilder().setKind(NodeKind_ADD).addChild(leaf)
	first := b.build().(*AstNode)
	b.addChild(leaf).setRelativeColumn(4).setDoubleValue(2)
	second := b.build().(*AstNode)
	if first.getChildCount() != 1 || first.getRelativeColumn() != 0 || second.getChildCount() != 2 || first.equals(second) {
		t.Errorf("builder reuse changed a built message")
	}
	b.clearChild().clearValue().clearRelativeColumn()
	if b.getChildCount() != 0 || b.getValueCase() != AstNode_ValueCase_VALUE_NOT_SET || second.getChildCount() != 2 {
		t.Errorf("clear")
	}
	b.clear()
	if !b.build().(*AstNode).equals(AstNode_getDefaultInstance()) {
		t.Errorf("clear() did not reset the builder")
	}

	// Oneofs: the last member set wins.
	sf := SourceFileProto_newBuilder().setPreloadedContents("x").setStubFile(true)
	if sf.getLoaderCase() != SourceFileProto_LoaderCase_STUB_FILE || sf.getPreloadedContents() != "" || !sf.getStubFile() {
		t.Errorf("oneof: %s", sf.toString())
	}
	sf.getFileOnDiskBuilder().setCharset("c")
	if sf.getLoaderCase() != SourceFileProto_LoaderCase_FILE_ON_DISK || sf.getStubFile() || sf.getFileOnDisk().getCharset() != "c" || !sf.hasFileOnDisk() {
		t.Errorf("oneof builder: %s", sf.toString())
	}
	switch sf.build().(*SourceFileProto).getLoaderCase() {
	case SourceFileProto_LoaderCase_FILE_ON_DISK:
	default:
		t.Errorf("switch on a oneof case")
	}
	if SourceFileProto_getDefaultInstance().getLoaderCase() != SourceFileProto_LoaderCase_LOADER_NOT_SET {
		t.Errorf("LOADER_NOT_SET")
	}

	// proto2 defaults and presence.
	r := Requirement_getDefaultInstance()
	if r.getType() != Requirement_Type_CUSTOM || r.getSeverity() != Requirement_Severity_WARNING ||
		r.getTypeMatchingStrategy() != Requirement_TypeMatchingStrategy_LOOSE || !r.getReportLooseTypeViolations() ||
		r.hasSeverity() || r.hasErrorMessage() || r.getErrorMessage() != "" || r.getValueList().size() != 0 {
		t.Errorf("proto2 defaults")
	}
	r2 := Requirement_newBuilder().setSeverity(Requirement_Severity_WARNING).setErrorMessage("").build().(*Requirement)
	if !r2.hasSeverity() || !r2.hasErrorMessage() || r2.getSerializedSize() != 4 || r2.equals(r) {
		t.Errorf("proto2 explicit presence: size %d", r2.getSerializedSize())
	}
	// proto3: default values are not present, except with `optional`.
	st := JSCompilerStateProto_newBuilder().setUniqueNameId(0).setIdGeneratorMap("")
	if !st.hasIdGeneratorMap() || st.build().(*JSCompilerStateProto).getSerializedSize() != 2 {
		t.Errorf("proto3 presence")
	}

	// mergeFrom and toBuilder, as CheckConformance uses them.
	base := pbTestConformance_().getRequirement(0)
	rb := base.toBuilder().(*Requirement_Builder)
	rb.mergeFrom_Requirement(Requirement_newBuilder().setErrorMessage("new").addAllValue(pbStrs_("more")).build().(*Requirement))
	if rb.getErrorMessage() != "new" || rb.getValueCount() != 3 || rb.getValue(2) != "more" || base.getValueCount() != 2 || rb.getRuleId() != "closure:eval" {
		t.Errorf("mergeFrom: %s", rb.toString())
	}
	rb.clearWhitelist().addAllWhitelist(pbStrs_("q", "r")).clearAllowlistEntry()
	if rb.getWhitelistCount() != 2 || rb.getWhitelistList().get(1) != any("r") || rb.getAllowlistEntryList().size() != 0 || base.getWhitelistCount() != 1 {
		t.Errorf("clear/addAll: %s", rb.toString())
	}
	var names []string
	for it := Requirement_newBuilder().setExtends("x").addAllValue(pbStrs_("v")).build().(*Requirement).getAllFields().keySet().iterator(); it.hasNext(); {
		names = append(names, it.next().(*Descriptors_FieldDescriptor).getName())
	}
	if strings.Join(names, ",") != "value,extends" {
		t.Errorf("getAllFields: %v", names)
	}

	// Repeated message builders.
	tp := TypePool_newBuilder()
	tp.addTypeBuilder().getObjectBuilder().setClosureAssert(true)
	tp.addType(TypeProto_getDefaultInstance())
	tp.getTypeBuilder(1).getUnionBuilder().addUnionMember(3)
	tp.removeType(0)
	pool := tp.build().(*TypePool)
	if pool.getTypeCount() != 1 || pool.getType(0).getKindCase() != TypeProto_KindCase_UNION || pool.getType(0).getUnion().getUnionMember(0) != 3 ||
		!pool.getType(0).hasUnion() || pool.getType(0).hasObject() || pool.hasDebugInfo() || pool.getTypeList().get(0) != any(pool.getType(0)) {
		t.Errorf("repeated builders: %s", pool.toString())
	}
}

func TestProtoEnums(t *testing.T) {
	facts := map[string]string{}
	for _, row := range pbLines_(t, "facts.tsv") {
		facts[row[0]] = row[1]
	}
	expect := func(key, got string) {
		t.Helper()
		if got != facts[key] {
			t.Errorf("%s: got %q, want %q", key, got, facts[key])
		}
	}
	expect("bytestring_hash", istr_(int64(ByteString_copyFrom([]int8{1, 2, -1}).hashCode())))
	expect("bytestring_hash_empty", istr_(int64(ByteString_EMPTY.hashCode())))
	expect("escape", TextFormat_escapeBytes(ByteString_copyFrom([]int8{0, 7, 8, 9, 10, 11, 12, 13, 27, 34, 39, 92, 127, -56, 'a'})))
	expect("unrecognized_ordinal", istr_(int64(NodeKind_UNRECOGNIZED.ordinal())))
	expect("nodekind_count", istr_(int64(len(NodeKind_values()))))
	expect("kindcase", istr_(int64(TypeProto_KindCase_valueOf("UNION").getNumber()))+","+istr_(int64(TypeProto_KindCase_KIND_NOT_SET.ordinal()))+","+
		TypeProto_KindCase_forNumber(2).toString())
	var cases []string
	for _, c := range SourceFileProto_LoaderCase_values() {
		cases = append(cases, c.name())
	}
	expect("loadercase", "["+strings.Join(cases, ", ")+"]")
	expect("sizes", istr_(int64(pbTestTypedAst_().getSerializedSize()))+","+istr_(int64(pbTestConformance_().getSerializedSize())))
	d := Requirement_getDefaultInstance()
	expect("req_type_default", d.getType().toString()+","+d.getSeverity().toString()+","+d.getTypeMatchingStrategy().toString()+","+
		boolstr_(d.getReportLooseTypeViolations()))
	txt := func(v float64) string {
		return strings.TrimSpace(TextFormat_printer().printToString(AstNode_newBuilder().setDoubleValue(v)))
	}
	expect("float_text", txt(1e21)+"|"+txt(100)+"|"+txt(1e-5))

	if NodeProperty_forNumber(NodeProperty_ARROW_FN.getNumber()) != NodeProperty_ARROW_FN || NodeProperty_forNumber(-5) != nil {
		t.Errorf("forNumber")
	}
	for i, v := range NodeProperty_values() {
		if int(v.ordinal()) != i || v != NodeProperty_UNRECOGNIZED && NodeProperty_forNumber(v.getNumber()) != v {
			t.Errorf("values()[%d] = %s", i, v.name())
		}
	}
	if FeatureProto_valueOf("FEATURE_GETTER") != FeatureProto_FEATURE_GETTER || FeatureProto_FEATURE_GETTER.getNumber() != 3 ||
		FeatureProto_FEATURE_GETTER_VALUE != 3 || FeatureProto_FEATURE_GETTER.toString() != "FEATURE_GETTER" {
		t.Errorf("valueOf")
	}
	if class, msg := pbThrown_(func() { FeatureProto_valueOf("NOPE") }); class != "java.lang.IllegalArgumentException" ||
		msg != "No enum constant com.google.javascript.jscomp.FeatureProto.NOPE" {
		t.Errorf("valueOf(NOPE): %s %s", class, msg)
	}
	if InstrumentationPoint_Type_valueOf("BRANCH").getNumber() != 2 || InstrumentationPoint_Type_BRANCH.ordinal() != 1 {
		t.Errorf("proto2 enum numbering")
	}
	if Object_getClass(NodeKind_ADD).getName() != "com.google.javascript.jscomp.serialization.NodeKind" ||
		Object_getClass(AstNode_getDefaultInstance()).getName() != "com.google.javascript.jscomp.serialization.AstNode" {
		t.Errorf("class names")
	}
}

func TestProtoStreams(t *testing.T) {
	// parseDelimitedFrom reads exactly one message and leaves the rest.
	data := pbFixture_(t, "delimited.bin")
	in := newPbTestIn_(data)
	st := JSCompilerStateProto_parseDelimitedFrom(&in.InputStream)
	if !st.equals(pbTestState_()) {
		t.Errorf("first delimited message differs: %s", st.toString())
	}
	if e := JSCompilerStateProto_parseDelimitedFrom(&in.InputStream); e == nil || !e.equals(JSCompilerStateProto_getDefaultInstance()) {
		t.Errorf("second delimited message")
	}
	cb := ConformanceConfig_newBuilder()
	if !cb.mergeDelimitedFrom(&in.InputStream) || !cb.build().(*ConformanceConfig).equals(pbTestConformance_()) {
		t.Errorf("third delimited message")
	}
	if !bytes.Equal(in.data, []byte{1, 2, 3}) {
		t.Errorf("parseDelimitedFrom read too far: %x left", in.data)
	}
	if JSCompilerStateProto_parseDelimitedFrom(&newPbTestIn_(nil).InputStream) != nil {
		t.Errorf("parseDelimitedFrom at end of stream should return null")
	}
	out := newPbTestOut_()
	pbTestState_().writeDelimitedTo(&out.OutputStream)
	JSCompilerStateProto_getDefaultInstance().writeDelimitedTo(&out.OutputStream)
	pbTestConformance_().writeDelimitedTo(&out.OutputStream)
	if !bytes.Equal(out.data, data[:len(data)-3]) {
		t.Errorf("writeDelimitedTo differs")
	}
	out = newPbTestOut_()
	pbTestConformance_().writeTo(&out.OutputStream)
	if !bytes.Equal(out.data, pbFixture_(t, "conformance.bin")) {
		t.Errorf("writeTo differs")
	}

	// The loop of TypedAstDeserializer over a TypedAst.List stream.
	list := pbFixture_(t, "typedast_list.bin")
	coded := CodedInputStream_newInstance(&newPbTestIn_(list).InputStream)
	b := TypedAst_newBuilder()
	n := 0
	want := pbTestTypedAst_()
	for !coded.isAtEnd() {
		tag := coded.readTag()
		if WireFormat_getTagFieldNumber(tag) != 1 || WireFormat_getTagWireType(tag) != 2 {
			t.Fatalf("tag %d", tag)
		}
		coded.readMessage(b, ExtensionRegistry_getEmptyRegistry().up_ExtensionRegistryLite())
		ta := b.build().(*TypedAst)
		b.clear()
		coded.resetSizeCounter()
		if n == 1 != ta.equals(TypedAst_getDefaultInstance()) || n != 1 && !ta.equals(want) {
			t.Errorf("TypedAst %d differs", n)
		}
		n++
	}
	if n != 3 || coded.readTag() != 0 {
		t.Errorf("read %d TypedAsts", n)
	}
	if l := pbParseStream_(TypedAst_List_desc_, &newPbTestIn_(list).InputStream).(*TypedAst_List); l.getTypedAstsCount() != 3 || !l.getTypedAsts(2).equals(want) {
		t.Errorf("parseFrom(InputStream)")
	}

	// Lazily parsed scripts, as ScriptNodeDeserializer reads them.
	script := want.getCodeAst(0).getScript()
	stream := script.newCodedInput()
	stream.setRecursionLimit(math.MaxInt32)
	if node := AstNode_parseFrom(stream, ExtensionRegistry_getEmptyRegistry().up_ExtensionRegistryLite()); !node.equals(pbTestRoot_()) {
		t.Errorf("AstNode.parseFrom(CodedInputStream)")
	}

	// Nesting deeper than the recursion limit is an error unless the limit is raised.
	deep := pbFixture_(t, "astnode_deep.bin")
	if class, msg := pbThrown_(func() { pbParse_(AstNode_desc_, deep, false) }); class != "com.google.protobuf.InvalidProtocolBufferException" ||
		!strings.Contains(msg, "too many levels of nesting") {
		t.Errorf("deep parse: %s %s", class, msg)
	}
	stream = (&ByteString{b_: deep}).newCodedInput()
	if old := stream.setRecursionLimit(1000); old != 100 {
		t.Errorf("default recursion limit %d", old)
	}
	node := AstNode_parseFrom(stream, nil)
	depth := 0
	for node.getChildCount() > 0 {
		node = node.getChild(0)
		depth++
	}
	if depth != 150 {
		t.Errorf("depth %d", depth)
	}

	// Truncated and malformed input.
	for _, bad := range [][]byte{list[:len(list)-1], {0x0a, 0x05, 1}, {0x00}, {0x0f}, {0x08, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}} {
		if class, _ := pbThrown_(func() { pbParse_(TypedAst_List_desc_, bad, false) }); class != "com.google.protobuf.InvalidProtocolBufferException" {
			t.Errorf("parse of %x threw %q", bad, class)
		}
	}
	if class, msg := pbThrown_(func() { pbParse_(SourceFileProto_desc_, []byte{0x0a, 0x02, 0xff, 0xfe}, false) }); class != "com.google.protobuf.InvalidProtocolBufferException" || msg != "Protocol message had invalid UTF-8." {
		t.Errorf("invalid UTF-8: %s %s", class, msg)
	}
}

func TestProtoByteString(t *testing.T) {
	out := ByteString_newOutput(4)
	for _, c := range []int32{0x68, 0xc3, 0xa9, 0x1ff} {
		out.write(c)
	}
	bs := out.toByteString()
	if bs.size() != 4 || bs.byteAt(1) != -61 || bs.byteAt(3) != -1 || bs.toStringUtf8()[:3] != "h\u00e9" || bs.isEmpty() {
		t.Errorf("ByteString.Output: %s", bs.toString())
	}
	if !bs.equals(ByteString_copyFrom([]int8{0x68, -61, -87, -1})) || bs.equals(ByteString_EMPTY) || bs.hashCode() != ByteString_copyFrom(bs.toByteArray()).hashCode() {
		t.Errorf("ByteString equality")
	}
	src := []int8{1, 2, 3}
	cp := ByteString_copyFrom(src)
	src[0] = 9
	arr := cp.toByteArray()
	arr[1] = 9
	if cp.byteAt(0) != 1 || cp.byteAt(1) != 2 {
		t.Errorf("ByteString is not immutable")
	}
	if ByteString_copyFrom([]int8{}) != ByteString_EMPTY || ByteString_copyFromUtf8("") != ByteString_EMPTY {
		t.Errorf("empty ByteStrings should be EMPTY")
	}
	if !cp.substring_int_int(1, 3).equals(ByteString_copyFrom([]int8{2, 3})) || !cp.concat(cp).startsWith(cp) || cp.concat(cp).size() != 6 {
		t.Errorf("substring/concat")
	}
	n := 0
	for it := cp.iterator(); it.hasNext(); n++ {
		if it.next() != any(int8(n+1)) {
			t.Errorf("iterator")
		}
	}
	// Bytes fields parsed from a caller's array do not alias it.
	raw := LazyAst_newBuilder().setScript(cp).build().(*LazyAst).toByteArray()
	parsed := pbParse_(LazyAst_desc_, pbU8_(raw), false).(*LazyAst)
	for i := range raw {
		raw[i] = 0
	}
	if !parsed.getScript().equals(cp) {
		t.Errorf("parsed bytes field aliases the input array")
	}
}
