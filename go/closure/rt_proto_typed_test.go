package closure

// Tests of what the Java fixtures of rt_proto_test.go do not reach: every scalar kind set,
// cleared and read at its default, oneof switching, the repeated-field operations, merging,
// unknown fields and builders reused after build().
//
// Each test writes a transcript (bytes, text format, hash codes, accessor results, exception
// classes) and compares it with testdata/proto/typed_<name>.txt. Those files were written by
// this same test code running on the runtime as it was before field storage became typed (one
// `any` slot per field, which the Java fixtures had validated), so the tests hold the typed
// storage to the behaviour of the boxed one. JSGO_PB_GOLDEN=1 rewrites the files.

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"
)

type pbRec_ struct {
	t  *testing.T
	sb strings.Builder
}

func (r *pbRec_) p(format string, a ...any) { fmt.Fprintf(&r.sb, format+"\n", a...) }

func pbOneLine_(s string) string { return strings.ReplaceAll(strings.TrimRight(s, "\n"), "\n", "; ") }

// msg records a message, or a builder and the message it builds: bytes, text, hash, size. A
// message is also parsed back from its bytes, which must give an equal message with the same bytes.
func (r *pbRec_) msg(label string, v any) {
	r.t.Helper()
	if b, ok := v.(pbB_); ok {
		r.p("%s: builder {%s}", label, pbOneLine_(b.pbb_().toString()))
		v = b.pbb_().build()
	}
	m := v.(pbM_).pbm_()
	data := m.bytes_()
	r.p("%s: %x | {%s} | hash %d | size %d", label, data, pbOneLine_(m.toString()), m.hashCode(), m.getSerializedSize())
	back := pbParse_(m.d_, data, false).(pbM_).pbm_()
	if !m.equals(back) || !back.equals(v) || !bytes.Equal(back.bytes_(), data) || back.hashCode() != m.hashCode() {
		r.t.Errorf("%s: parsed back as {%s} %x", label, pbOneLine_(back.toString()), back.bytes_())
	}
}

func (r *pbRec_) thrown(label string, f func()) {
	class, msg := pbThrown_(f)
	r.p("%s: %s: %s", label, class, msg)
}

func pbListStr_(l List) string {
	var parts []string
	for _, v := range toSlice_(l) {
		switch x := v.(type) {
		case *ByteString:
			parts = append(parts, fmt.Sprintf("bytes(%x)", x.b_))
		case pbM_:
			parts = append(parts, "{"+pbOneLine_(x.pbm_().toString())+"}")
		case pbB_:
			parts = append(parts, "builder{"+pbOneLine_(x.pbb_().toString())+"}")
		default:
			parts = append(parts, fmt.Sprintf("%T(%v)", v, v))
		}
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func (r *pbRec_) check(name string) {
	r.t.Helper()
	path := filepath.Join("testdata", "proto", "typed_"+name+".txt")
	got := pbAscii_(r.sb.String())
	if os.Getenv("JSGO_PB_GOLDEN") != "" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			r.t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		r.t.Fatal(err)
	}
	if got == string(want) {
		return
	}
	g, w := strings.Split(got, "\n"), strings.Split(string(want), "\n")
	for i := 0; i < len(g) || i < len(w); i++ {
		var a, b string
		if i < len(g) {
			a = g[i]
		}
		if i < len(w) {
			b = w[i]
		}
		if a != b {
			r.t.Fatalf("%s line %d:\n got %s\nwant %s", path, i+1, a, b)
		}
	}
}

// pbAscii_ keeps the transcripts text: every byte that is not printable ASCII or a newline as \xNN.
func pbAscii_(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if c := s[i]; c == '\n' || c >= 0x20 && c < 0x7f {
			sb.WriteByte(c)
		} else {
			fmt.Fprintf(&sb, "\\x%02x", c)
		}
	}
	return sb.String()
}

func pbHex_(s string) []byte {
	b, err := hex.DecodeString(strings.ReplaceAll(s, " ", ""))
	if err != nil {
		panic(err)
	}
	return b
}

func pbAny2_(v ...any) Iterable { return listOf_(v) }

// TestProtoTypedScalars: every scalar kind the compiler's messages use (enum, uint32, sint32,
// int32, int64, double, bool, string, bytes), with and without explicit presence and with
// proto2 defaults: read at the default, set, set to the default, cleared.
func TestProtoTypedScalars(t *testing.T) {
	r := &pbRec_{t: t}
	ast := func(b *AstNode_Builder) string {
		return fmt.Sprintf("kind=%d/%d line=%d col=%d orig=%d props=%d type=%v/%d file=%d str=%v/%d dbl=%v/%v tmpl=%v jsdoc=%v case=%d",
			b.getKind(), b.getKindValue(), b.getRelativeLine(), b.getRelativeColumn(), b.getOriginalNamePointer(), b.getBooleanProperties(),
			b.hasType(), b.getType(), b.getSourceFile(), b.hasStringValuePointer(), b.getStringValuePointer(),
			b.hasDoubleValue(), b.getDoubleValue(), b.hasTemplateStringValue(), b.hasJsdoc(), b.getValueCase())
	}
	b := AstNode_newBuilder()
	r.p("ast defaults: %s", ast(b))
	r.msg("ast empty", b)
	b.setKind(NodeKind_IDENTIFIER).setRelativeLine(-3).setRelativeColumn(1 << 20).setOriginalNamePointer(-1).
		setBooleanProperties(math.MinInt64).setType(0).setSourceFile(math.MaxInt32)
	r.p("ast set: %s", ast(b))
	r.msg("ast set", b)
	m := b.build().(*AstNode)
	r.p("ast message: kind=%d/%d line=%d col=%d orig=%d props=%d type=%v/%d file=%d case=%d", m.getKind(), m.getKindValue(), m.getRelativeLine(),
		m.getRelativeColumn(), m.getOriginalNamePointer(), m.getBooleanProperties(), m.hasType(), m.getType(), m.getSourceFile(), m.getValueCase())
	b.setKindValue(9999)
	r.p("ast unknown kind: %s", ast(b))
	r.msg("ast unknown kind", b)
	for i, clear := range []func() *AstNode_Builder{b.clearKind, b.clearRelativeLine, b.clearRelativeColumn, b.clearOriginalNamePointer,
		b.clearBooleanProperties, b.clearType, b.clearSourceFile} {
		clear()
		r.p("ast clear %d: %s", i, ast(b))
		r.msg(fmt.Sprintf("ast clear %d", i), b)
	}
	// Values equal to the default: written only where the field has presence.
	b.setKindValue(0).setRelativeLine(0).setRelativeColumn(0).setOriginalNamePointer(0).setBooleanProperties(0).setSourceFile(0)
	r.msg("ast zeros", b)
	b.setType(0)
	r.msg("ast optional zero", b)
	b.setType(-1).setRelativeLine(math.MinInt32).setRelativeColumn(-1).setBooleanProperties(-1).setSourceFile(-1)
	r.msg("ast negative", b)
	for _, d := range []float64{0, math.Copysign(0, -1), 1.5, -1e300, math.Inf(1), math.Inf(-1), math.NaN(), math.SmallestNonzeroFloat64} {
		b = AstNode_newBuilder().setDoubleValue(d)
		r.p("double %v: has=%v get=%v", d, b.hasDoubleValue(), b.getDoubleValue())
		r.msg(fmt.Sprintf("double %v", d), b)
	}
	r.msg("double cleared", b.clearDoubleValue())
	r.thrown("null enum", func() { AstNode_newBuilder().setKind(0) })

	// bool, bytes, a message field.
	o := ObjectTypeProto_newBuilder()
	r.p("object defaults: inv=%v uuid=%d empty=%v keep=%v debug=%v", o.getIsInvalidating(), o.getUuid().size(), o.getUuid() == ByteString_EMPTY,
		o.getPropertiesKeepOriginalName(), o.hasDebugInfo())
	o.setIsInvalidating(true).setUuid(ByteString_copyFrom([]int8{0, -1, 7})).setClosureAssert(true).setMarkedConstructor(false)
	r.p("object set: inv=%v uuid=%x assert=%v marked=%v", o.getIsInvalidating(), o.getUuid().b_, o.getClosureAssert(), o.getMarkedConstructor())
	r.msg("object set", o)
	om := o.build().(*ObjectTypeProto)
	r.p("object message: inv=%v uuid=%x assert=%v debug=%v default debug=%v", om.getIsInvalidating(), om.getUuid().b_, om.getClosureAssert(),
		om.hasDebugInfo(), om.getDebugInfo() == ObjectTypeProto_DebugInfo_getDefaultInstance())
	o.setUuid(ByteString_copyFrom([]int8{}))
	r.p("object empty uuid: %d empty=%v", o.getUuid().size(), o.getUuid() == ByteString_EMPTY)
	r.msg("object empty uuid", o)
	o.setUuid(ByteString_copyFromUtf8("u")).clearIsInvalidating().clearClosureAssert()
	r.msg("object bools cleared", o)
	o.clearUuid()
	r.p("object uuid cleared: %d", o.getUuid().size())
	r.msg("object uuid cleared", o)
	r.thrown("null bytes", func() { ObjectTypeProto_newBuilder().setUuid(nil) })
	dm := ObjectTypeProto_newBuilder().build().(*ObjectTypeProto)
	r.p("object default message: uuid empty=%v", dm.getUuid() == ByteString_EMPTY)

	// string, without presence and with (oneof member, optional).
	s := SourceFileProto_newBuilder()
	r.p("file defaults: name=%q pre=%v/%q kind=%d/%d", s.getFilename(), s.hasPreloadedContents(), s.getPreloadedContents(), s.getSourceKind(), s.getSourceKindValue())
	s.setFilename("a\u00e9.js").setSourceKind(SourceFileProto_SourceKind_EXTERN).setNumBytesPlusOne(300).setIsClosureUnawareCode(true)
	r.p("file set: name=%q bytes=%x kind=%d/%d", s.getFilename(), s.getFilenameBytes().b_, s.getSourceKind(), s.getSourceKindValue())
	r.msg("file set", s)
	s.setFilename("")
	r.msg("file empty name", s)
	s.setFilenameBytes(ByteString_copyFromUtf8("b.js")).setPreloadedContents("")
	r.p("file bytes name: name=%q pre=%v/%q", s.getFilename(), s.hasPreloadedContents(), s.getPreloadedContents())
	r.msg("file bytes name", s)
	s.clearFilename().clearSourceKind().clearPreloadedContents()
	r.p("file cleared: name=%q pre=%v", s.getFilename(), s.hasPreloadedContents())
	r.msg("file cleared", s)
	r.thrown("null string", func() { SourceFileProto_newBuilder().setFilename(nullstr_) })
	st := JSCompilerStateProto_newBuilder()
	r.p("state defaults: map=%v/%q stage=%d/%d id=%d", st.hasIdGeneratorMap(), st.getIdGeneratorMap(), st.getLifeCycleStage(), st.getLifeCycleStageValue(), st.getUniqueNameId())
	st.setIdGeneratorMap("").setLifeCycleStageValue(3).setUniqueNameId(-1).setTypeCheckingHasRun(true)
	r.p("state set: map=%v/%q stage=%d/%d id=%d", st.hasIdGeneratorMap(), st.getIdGeneratorMap(), st.getLifeCycleStage(), st.getLifeCycleStageValue(), st.getUniqueNameId())
	r.msg("state set", st)
	st.clearIdGeneratorMap().clearLifeCycleStage().clearUniqueNameId().clearTypeCheckingHasRun()
	r.msg("state cleared", st)

	// proto2: every singular field has presence, and some have defaults that are not zero.
	q := Requirement_newBuilder()
	req := func() string {
		return fmt.Sprintf("msg=%v/%q type=%v/%d sev=%v/%d strat=%v/%d loose=%v/%v ext=%v/%v", q.hasErrorMessage(), q.getErrorMessage(), q.hasType(), q.getType(),
			q.hasSeverity(), q.getSeverity(), q.hasTypeMatchingStrategy(), q.getTypeMatchingStrategy(),
			q.hasReportLooseTypeViolations(), q.getReportLooseTypeViolations(), q.hasAllowExtendingValue(), q.getAllowExtendingValue())
	}
	r.p("requirement defaults: %s", req())
	r.msg("requirement empty", q)
	dq := Requirement_getDefaultInstance()
	r.p("requirement default message: sev=%v/%d strat=%d loose=%v type=%d", dq.hasSeverity(), dq.getSeverity(), dq.getTypeMatchingStrategy(), dq.getReportLooseTypeViolations(), dq.getType())
	q.setErrorMessage("").setType(Requirement_Type_CUSTOM).setSeverity(Requirement_Severity_WARNING).
		setTypeMatchingStrategy(Requirement_TypeMatchingStrategy_LOOSE).setReportLooseTypeViolations(true).setAllowExtendingValue(false)
	r.p("requirement set to defaults: %s", req())
	r.msg("requirement set to defaults", q)
	q.setSeverity(Requirement_Severity_ERROR).setReportLooseTypeViolations(false).setErrorMessage("e")
	r.p("requirement set: %s", req())
	r.msg("requirement set", q)
	q.clearSeverity().clearReportLooseTypeViolations().clearErrorMessage().clearType().clearTypeMatchingStrategy().clearAllowExtendingValue()
	r.p("requirement cleared: %s", req())
	r.msg("requirement cleared", q)
	om2 := Mapping_OriginalMapping_newBuilder()
	r.p("mapping defaults: file=%v/%q line=%v/%d prec=%v/%d", om2.hasOriginalFile(), om2.getOriginalFile(), om2.hasLineNumber(), om2.getLineNumber(), om2.hasPrecision(), om2.getPrecision())
	om2.setPrecision(Mapping_OriginalMapping_Precision_EXACT).setLineNumber(0)
	r.p("mapping set: line=%v/%d prec=%v/%d", om2.hasLineNumber(), om2.getLineNumber(), om2.hasPrecision(), om2.getPrecision())
	r.msg("mapping set", om2)
	r.check("scalars")
}

// TestProtoTypedOneof: setting a member of a oneof clears the others; clearing a member that is
// not the one set leaves the oneof alone.
func TestProtoTypedOneof(t *testing.T) {
	r := &pbRec_{t: t}
	b := AstNode_newBuilder().setKind(NodeKind_NUMBER_LITERAL)
	state := func(label string) {
		r.p("%s: case=%d str=%v/%d dbl=%v/%v tmpl=%v/{%s}", label, b.getValueCase(), b.hasStringValuePointer(), b.getStringValuePointer(),
			b.hasDoubleValue(), b.getDoubleValue(), b.hasTemplateStringValue(), pbOneLine_(b.getTemplateStringValue().toString()))
		r.msg(label, b)
	}
	state("none")
	b.setStringValuePointer(0)
	state("string 0")
	b.setDoubleValue(2.5)
	state("double")
	b.clearStringValuePointer()
	state("clear the other member")
	b.setTemplateStringValue(TemplateStringValue_newBuilder().setRawStringPointer(4).build().(*TemplateStringValue))
	state("template")
	b.setStringValuePointer(9)
	state("string 9")
	b.setTemplateStringValue_Builder(TemplateStringValue_newBuilder().setCookedStringPointer(1))
	state("template from builder")
	b.getTemplateStringValueBuilder().setRawStringPointer(8)
	state("template builder changed")
	b.setDoubleValue(0)
	state("double 0")
	b.getTemplateStringValueBuilder()
	state("empty template builder")
	b.setDoubleValue(1)
	b.mergeTemplateStringValue(TemplateStringValue_newBuilder().setRawStringPointer(2).build().(*TemplateStringValue))
	state("merge template over double")
	b.clearValue()
	state("clear oneof")
	b.setDoubleValue(3).clearDoubleValue()
	state("clear the member set")

	s := SourceFileProto_newBuilder().setFilename("f")
	file := func(label string) {
		r.p("%s: case=%d pre=%v/%q disk=%v zip=%v stub=%v/%v", label, s.getLoaderCase(), s.hasPreloadedContents(), s.getPreloadedContents(),
			s.hasFileOnDisk(), s.hasZipEntry(), s.hasStubFile(), s.getStubFile())
		r.msg(label, s)
	}
	file("file none")
	s.setPreloadedContents("x")
	file("file preloaded")
	s.setFileOnDisk(SourceFileProto_FileOnDisk_newBuilder().setActualPath("/p"))
	file("file on disk")
	s.getZipEntryBuilder().setZipPath("z")
	file("file zip builder")
	s.setStubFile(false)
	file("file stub false")
	s.setPreloadedContents("")
	file("file preloaded empty")
	s.clearStubFile()
	file("file clear the other member")
	s.clearLoader()
	file("file clear oneof")
	m := s.setStubFile(true).build().(*SourceFileProto)
	r.p("file message: case=%d stub=%v/%v pre=%v disk=%v default disk=%v", m.getLoaderCase(), m.hasStubFile(), m.getStubFile(), m.hasPreloadedContents(),
		m.hasFileOnDisk(), m.getFileOnDisk() == SourceFileProto_FileOnDisk_getDefaultInstance())
	// On the wire the last member wins.
	two := append(SourceFileProto_newBuilder().setPreloadedContents("first").build().(*SourceFileProto).pbm_().bytes_(),
		SourceFileProto_newBuilder().setStubFile(true).build().(*SourceFileProto).pbm_().bytes_()...)
	pm := SourceFileProto_parseFrom_ByteString(pbBS_(two))
	r.p("file parsed twice: case=%d pre=%v stub=%v", pm.getLoaderCase(), pm.hasPreloadedContents(), pm.hasStubFile())
	r.msg("file parsed twice", pm)
	tp := TypeProto_newBuilder()
	tp.getObjectBuilder().setIsInvalidating(true)
	r.p("type object: case=%d", tp.getKindCase())
	tp.getUnionBuilder().addUnionMember(3)
	r.p("type union: case=%d object=%v union=%v", tp.getKindCase(), tp.hasObject(), tp.hasUnion())
	r.msg("type union", tp)
	r.check("oneof")
}

// TestProtoTypedRepeated: add, addAll, set, insert, remove and clear on repeated fields of
// every element type (int32 packed, string, bytes, enum, message), and the index errors.
func TestProtoTypedRepeated(t *testing.T) {
	r := &pbRec_{t: t}
	o := ObjectTypeProto_newBuilder()
	r.p("ints empty: %d %s", o.getInstanceTypeCount(), pbListStr_(o.getInstanceTypeList()))
	o.addInstanceType(1).addInstanceType(-1).addAllInstanceType(pbInts_(300, 0, 70000)).addPrototype(5)
	r.p("ints: %d %s get(2)=%d", o.getInstanceTypeCount(), pbListStr_(o.getInstanceTypeList()), o.getInstanceType(2))
	r.msg("ints", o)
	o.setInstanceType(1, 42).setInstanceType(4, -5)
	r.msg("ints set", o)
	om := o.build().(*ObjectTypeProto)
	r.p("ints message: %d %s get(4)=%d", om.getInstanceTypeCount(), pbListStr_(om.getInstanceTypeList()), om.getInstanceType(4))
	r.thrown("ints get -1", func() { o.getInstanceType(-1) })
	r.thrown("ints get 5", func() { om.getInstanceType(5) })
	r.thrown("ints set 5", func() { o.setInstanceType(5, 1) })
	r.thrown("ints addAll null", func() { o.addAllInstanceType(nil) })
	r.thrown("ints addAll null element", func() { o.addAllInstanceType(pbAny2_(int32(1), nil)) })
	r.msg("ints after errors", o)
	o.clearInstanceType()
	r.p("ints cleared: %d %s", o.getInstanceTypeCount(), pbListStr_(o.getInstanceTypeList()))
	r.msg("ints cleared", o)

	st := JSCompilerStateProto_newBuilder()
	st.addExterns("a").addExterns("").addAllExterns(pbStrs_("c", "d\u00e9")).addExternsBytes(ByteString_copyFromUtf8("e"))
	r.p("strings: %d %s get(3)=%q bytes(3)=%x", st.getExternsCount(), pbListStr_(st.getExternsList()), st.getExterns(3), st.getExternsBytes(3).b_)
	st.setExterns(0, "A")
	r.msg("strings", st)
	r.thrown("strings add null", func() { st.addExterns(nullstr_) })
	r.thrown("strings set null", func() { st.setExterns(0, nullstr_) })
	r.thrown("strings get 9", func() { st.getExterns(9) })
	r.thrown("strings bytes 9", func() { st.getExternsBytes(9) })
	st.clearExterns()
	st.addAllowableFeatures(FeatureProto_forNumber(1)).addAllowableFeaturesValue(555).addAllowableFeatures(FeatureProto_forNumber(0)).
		addAllAllowableFeatures(pbAny2_(FeatureProto_forNumber(2), FeatureProto_forNumber(3))).addAllAllowableFeaturesValue(pbInts_(7, 8000))
	r.p("enums: %d %s values %s get(1)=%d value(1)=%d get(0)=%d", st.getAllowableFeaturesCount(), pbListStr_(st.getAllowableFeaturesList()),
		pbListStr_(st.getAllowableFeaturesValueList()), st.getAllowableFeatures(1), st.getAllowableFeaturesValue(1), st.getAllowableFeatures(0))
	st.setAllowableFeatures(0, FeatureProto_forNumber(4)).setAllowableFeaturesValue(2, 9)
	r.msg("enums", st)
	sm := st.build().(*JSCompilerStateProto)
	r.p("enums message: %d %s values %s get(1)=%d value(6)=%d", sm.getAllowableFeaturesCount(), pbListStr_(sm.getAllowableFeaturesList()),
		pbListStr_(sm.getAllowableFeaturesValueList()), sm.getAllowableFeatures(1), sm.getAllowableFeaturesValue(6))
	r.thrown("enums get 7", func() { sm.getAllowableFeatures(7) })
	st.clearAllowableFeatures()
	r.msg("enums cleared", st)

	sp := StringPoolProto_newBuilder().addStrings(ByteString_EMPTY).addStrings(ByteString_copyFromUtf8("x")).
		addAllStrings(pbAny2_(ByteString_copyFrom([]int8{-1}), ByteString_copyFrom([]int8{})))
	sp.setStrings(1, ByteString_copyFromUtf8("yy"))
	r.p("bytes: %d %s get(2)=%x", sp.getStringsCount(), pbListStr_(sp.getStringsList()), sp.getStrings(2).b_)
	r.msg("bytes", sp)
	r.thrown("bytes add null", func() { sp.addStrings(nil) })
	r.thrown("bytes set null", func() { sp.setStrings(0, nil) })

	chunk := func(name string) *ChunkProto { return ChunkProto_newBuilder().setName(name).build().(*ChunkProto) }
	st = JSCompilerStateProto_newBuilder()
	st.addChunks(chunk("a")).addChunks_Builder(ChunkProto_newBuilder().setName("b")).addChunks_int_ChunkProto(0, chunk("c")).
		addChunks_int_Builder(3, ChunkProto_newBuilder().setName("d")).addAllChunks(pbAny2_(chunk("e"), chunk("f")))
	r.p("messages: %d %s get(2)={%s}", st.getChunksCount(), pbListStr_(st.getChunksList()), pbOneLine_(st.getChunks(2).toString()))
	r.msg("messages", st)
	st.setChunks(1, chunk("A")).setChunks_int_Builder(5, ChunkProto_newBuilder().setName("F").addDependencies(1)).removeChunks(0).removeChunks(4)
	r.msg("messages set and removed", st)
	st.getChunksBuilder(1).addInputIds("in")
	st.addChunksBuilder().setName("g")
	st.addChunksBuilder_int(0).setName("h")
	r.p("messages with builders: %d %s get(0)={%s} builders=%d", st.getChunksCount(), pbListStr_(st.getChunksList()),
		pbOneLine_(st.getChunks(0).toString()), st.getChunksBuilderList().size())
	r.msg("messages with builders", st)
	st.getChunksBuilderList().get(2).(*ChunkProto_Builder).setName("via list")
	sm = st.build().(*JSCompilerStateProto)
	r.p("messages message: %d %s get(2)={%s} orBuilder={%s}", sm.getChunksCount(), pbListStr_(sm.getChunksList()), pbOneLine_(sm.getChunks(2).toString()),
		pbOneLine_(sm.getChunksOrBuilder(0).(*ChunkProto).toString()))
	r.thrown("messages add null", func() { st.addChunks(nil) })
	r.thrown("messages set null", func() { st.setChunks(0, nil) })
	r.thrown("messages get 6", func() { sm.getChunks(6) })
	r.thrown("messages builder get 6", func() { st.getChunks(6) })
	r.thrown("messages remove 6", func() { st.removeChunks(6) })
	r.thrown("messages insert 7", func() { st.addChunks_int_ChunkProto(7, chunk("x")) })
	r.thrown("messages builder -1", func() { st.getChunksBuilder(-1) })
	r.msg("messages after errors", st)
	st.clearChunks()
	r.p("messages cleared: %d", st.getChunksCount())
	r.msg("messages cleared", st)
	r.check("repeated")
}

// TestProtoTypedMerge: mergeFrom(Message), mergeFoo(Foo), and the merge that parsing a
// singular message field twice is.
func TestProtoTypedMerge(t *testing.T) {
	r := &pbRec_{t: t}
	a := pbTestState_()
	other := JSCompilerStateProto_newBuilder().setUniqueNameId(7).addExterns("z.js").setIdGeneratorMap("m").
		setAccessorSummary(AccessorSummaryProto_newBuilder().
			addAccessors(AccessorSummaryEntryProto_newBuilder().setName("q").build().(*AccessorSummaryEntryProto)).build().(*AccessorSummaryProto)).
		addAllowableFeaturesValue(9).addChunks(ChunkProto_newBuilder().setName("second").build().(*ChunkProto)).build().(*JSCompilerStateProto)
	b := a.toBuilder().(*JSCompilerStateProto_Builder).mergeFrom_JSCompilerStateProto(other)
	r.msg("state merged", b)
	r.msg("state merged into empty", JSCompilerStateProto_newBuilder().mergeFrom_JSCompilerStateProto(other).mergeFrom_JSCompilerStateProto(JSCompilerStateProto_getDefaultInstance()))
	r.msg("state first unchanged", a)
	r.msg("state other unchanged", other)
	// A nested builder is merged into in place; a nested message is replaced by the merge.
	b = JSCompilerStateProto_newBuilder()
	b.getAccessorSummaryBuilder().setAssumeAlwaysGetterAndSetter(true)
	b.mergeAccessorSummary(other.getAccessorSummary())
	r.msg("merge into nested builder", b)
	b = JSCompilerStateProto_newBuilder().setAccessorSummary(a.getAccessorSummary()).mergeAccessorSummary(other.getAccessorSummary())
	r.msg("merge into nested message", b)
	r.msg("nested message unchanged", a.getAccessorSummary())
	b = JSCompilerStateProto_newBuilder().mergeAccessorSummary(other.getAccessorSummary())
	r.p("merge into nothing: same=%v", b.build().(*JSCompilerStateProto).getAccessorSummary() == other.getAccessorSummary())
	r.thrown("merge null", func() { JSCompilerStateProto_newBuilder().mergeAccessorSummary(nil) })
	r.thrown("mergeFrom null", func() { JSCompilerStateProto_newBuilder().mergeFrom_JSCompilerStateProto(nil) })

	// A oneof: the member set in the other message replaces the one set here.
	x := AstNode_newBuilder().setStringValuePointer(4).setType(2).addChild(pbTestLeaf_())
	y := AstNode_newBuilder().setDoubleValue(0).setRelativeLine(5).addChild(AstNode_getDefaultInstance()).build().(*AstNode)
	r.msg("ast merged", x.mergeFrom_AstNode(y))
	x = AstNode_newBuilder().setTemplateStringValue(TemplateStringValue_newBuilder().setRawStringPointer(1).build().(*TemplateStringValue))
	y = AstNode_newBuilder().setTemplateStringValue(TemplateStringValue_newBuilder().setCookedStringPointer(2).build().(*TemplateStringValue)).build().(*AstNode)
	r.msg("ast template merged", x.mergeFrom_AstNode(y))
	x = AstNode_newBuilder().setJsdoc(OptimizationJsdoc_newBuilder().addKind(JsdocTag_JSDOC_CONST).setLicenseTextPointer(1).build().(*OptimizationJsdoc))
	x.mergeJsdoc(OptimizationJsdoc_newBuilder().addKind(JsdocTag_JSDOC_NO_INLINE).setMeaningPointer(3).build().(*OptimizationJsdoc))
	r.msg("jsdoc merged", x)

	// Two serialized messages one after the other parse as their merge.
	p1 := AstNode_newBuilder().setKind(NodeKind_IDENTIFIER).setType(1).addChild(pbTestLeaf_()).
		setJsdoc(OptimizationJsdoc_newBuilder().addKind(JsdocTag_JSDOC_CONST).setLicenseTextPointer(1).build().(*OptimizationJsdoc)).build().(*AstNode)
	p2 := AstNode_newBuilder().setRelativeLine(9).setStringValuePointer(3).addChild(AstNode_newBuilder().setSourceFile(2).build().(*AstNode)).
		setJsdoc(OptimizationJsdoc_newBuilder().addKind(JsdocTag_JSDOC_NO_INLINE).setMeaningPointer(3).build().(*OptimizationJsdoc)).build().(*AstNode)
	both := append(append(p1.pbm_().bytes_(), p2.pbm_().bytes_()...), p1.pbm_().bytes_()...)
	r.msg("parsed concatenation", AstNode_parseFrom_ByteString(pbBS_(both)))
	r.msg("first of the concatenation unchanged", p1)
	// The text format merges into a builder that has fields already.
	tb := pbTestState_().toBuilder().(*JSCompilerStateProto_Builder)
	TextFormat_merge(strobj_("externs: \"t.js\" unique_name_id: 1 accessor_summary { accessors { name: \"t\" } } chunks { name: \"tc\" }"), tb)
	r.msg("text merged", tb)
	r.check("merge")
}

// TestProtoTypedUnknown: unknown fields are kept, in field-number order, through parsing,
// builders and merging, and count in equals and hashCode.
func TestProtoTypedUnknown(t *testing.T) {
	r := &pbRec_{t: t}
	// Unknown fields 100 (varint), 99 (bytes), 98 (fixed32), 97 (fixed64), 96 (group) around known ones, out of order.
	data := pbHex_("a006 2a" + "0803" + "9a06 02 6869" + "9506 01020304" + "20 05" + "8906 0102030405060708" + "8306 08 01 8406" + "a006 01")
	m := AstNode_parseFrom_ByteString(pbBS_(data))
	r.p("parsed: kind=%d line=%d", m.getKindValue(), m.getRelativeLine())
	r.msg("parsed", m)
	plain := AstNode_newBuilder().setKindValue(3).setRelativeLine(-3).build().(*AstNode)
	r.p("equals without unknown: %v %v; hash equal: %v", m.equals(plain), plain.equals(m), m.hashCode() == plain.hashCode())
	b := m.toBuilder().(*AstNode_Builder)
	r.msg("through a builder", b)
	r.p("builder's message equals: %v", m.equals(b.build()))
	b.setKindValue(4).addChild(m)
	r.msg("builder changed", b)
	r.msg("merged twice", AstNode_newBuilder().mergeFrom_AstNode(m).mergeFrom_AstNode(m).setSourceFile(1))
	r.msg("merged into known", plain.toBuilder().(*AstNode_Builder).mergeFrom_AstNode(m))
	b.clear()
	r.msg("builder cleared", b)
	r.msg("parsed unchanged", m)
	// A known field with the wrong wire type is unknown; so is a packed field of a known number given as fixed32.
	r.msg("wrong wire type", AstNode_parseFrom_ByteString(pbBS_(pbHex_("0a 01 00 0d 01000000 20 02"))))
	// A closed enum's unknown number goes to the unknown fields (proto2).
	rq := Requirement_parseFrom_ByteString(pbBS_(pbHex_("10 63 60 02 60 63 0a 01 65")))
	r.p("closed enum: type=%v/%d sev=%v/%d", rq.hasType(), rq.getType(), rq.hasSeverity(), rq.getSeverity())
	r.msg("closed enum", rq)
	r.thrown("truncated", func() { AstNode_parseFrom_ByteString(pbBS_(pbHex_("9a06 05 6869"))) })
	r.thrown("zero tag", func() { AstNode_parseFrom_ByteString(pbBS_(pbHex_("00 01"))) })
	r.thrown("end group", func() { AstNode_parseFrom_ByteString(pbBS_(pbHex_("8306 8c06"))) })
	r.thrown("bad wire type", func() { AstNode_parseFrom_ByteString(pbBS_(pbHex_("a606 01"))) })
	r.thrown("bad utf8", func() { SourceFileProto_parseFrom_ByteString(pbBS_(pbHex_("0a 02 c328"))) })
	r.check("unknown")
}

// TestProtoTypedBuilderReuse: a builder goes on after build(), a message's builders do not
// disturb the message or each other, and nested builders stay live until the next build().
func TestProtoTypedBuilderReuse(t *testing.T) {
	r := &pbRec_{t: t}
	chunk := func(name string) *ChunkProto { return ChunkProto_newBuilder().setName(name).build().(*ChunkProto) }
	b := JSCompilerStateProto_newBuilder().addExterns("a").addExterns("b").addChunks(chunk("1")).addChunks(chunk("2")).addChunks(chunk("3")).
		addAllowableFeaturesValue(1).setUniqueNameId(1)
	m1 := b.build().(*JSCompilerStateProto)
	b.addExterns("c").setUniqueNameId(2)
	m2 := b.build().(*JSCompilerStateProto)
	b.setExterns(0, "A").removeChunks(0).addChunks(chunk("4")).setAllowableFeaturesValue(0, 2)
	m3 := b.build().(*JSCompilerStateProto)
	b.clearExterns().addExterns("d").addChunks_int_ChunkProto(0, chunk("0"))
	m4 := b.build().(*JSCompilerStateProto)
	b.getChunksBuilder(1).setName("two")
	m5 := b.build().(*JSCompilerStateProto)
	b.getChunksBuilder(1).setName("TWO")
	m6 := b.build().(*JSCompilerStateProto)
	b.clear()
	r.msg("m1", m1)
	r.msg("m2", m2)
	r.msg("m3", m3)
	r.msg("m4", m4)
	r.msg("m5", m5)
	r.msg("m6", m6)
	r.msg("cleared", b)
	b.addExterns("again")
	r.msg("after clear", b)
	r.msg("m6 still", m6)

	// Two builders of one message, and the message's own builder after its build.
	base := JSCompilerStateProto_newBuilder().addExterns("x").addExterns("y").addExterns("z")
	base.setExterns(2, "Z")
	bm := base.build().(*JSCompilerStateProto)
	b1, b2 := bm.toBuilder().(*JSCompilerStateProto_Builder), bm.toBuilder().(*JSCompilerStateProto_Builder)
	b1.addExterns("one")
	b2.addExterns("two")
	base.addExterns("base")
	n1 := b1.build().(*JSCompilerStateProto)
	b1.addExterns("one more")
	b2.setExterns(0, "X2")
	r.msg("base message", bm)
	r.msg("base builder", base)
	r.msg("b1 first build", n1)
	r.msg("b1", b1)
	r.msg("b2", b2)
	// A parsed message's builder.
	pm := JSCompilerStateProto_parseFrom_ByteString(pbBS_(bm.pbm_().bytes_()))
	pb := pm.toBuilder().(*JSCompilerStateProto_Builder).addExterns("p")
	pb2 := pm.toBuilder().(*JSCompilerStateProto_Builder).addExterns("q")
	r.msg("parsed", pm)
	r.msg("parsed builder", pb)
	r.msg("parsed builder 2", pb2)

	// Nested builders: what they hold when the outer builder is built.
	tp := TypePool_newBuilder()
	nb := tp.addTypeBuilder()
	nb.getUnionBuilder().addUnionMember(1)
	db := tp.getDebugInfoBuilder()
	t1 := tp.build().(*TypePool)
	nb.getUnionBuilder().addUnionMember(2)
	db.addMismatchBuilder().setSourceRef("r")
	t2 := tp.build().(*TypePool)
	tp.addType(TypeProto_getDefaultInstance())
	nb.getUnionBuilder().setUnionMember(0, 9)
	t3 := tp.build().(*TypePool)
	r.msg("t1", t1)
	r.msg("t2", t2)
	r.msg("t3", t3)
	r.p("built twice: equal=%v same=%v", tp.build().(*TypePool).equals(t3), tp.build() == tp.build())
	// The builder's getters build the nested builders.
	r.p("builder getters: type(0)={%s} debug={%s} list=%s", pbOneLine_(tp.getType(0).toString()), pbOneLine_(tp.getDebugInfo().toString()), pbListStr_(tp.getTypeList()))
	// Builder to builder: newBuilder(prototype) and toBuilder of a message with nested messages.
	c := TypePool_newBuilder_TypePool(t3)
	c.getTypeBuilder(0).getUnionBuilder().addUnionMember(77)
	c.getDebugInfoBuilder().getMismatchBuilder(0).setSourceRef("changed")
	r.msg("copy changed", c)
	r.msg("t3 still", t3)
	r.check("builder_reuse")
}

func pbTestAllOf_(text string) pbM_ {
	b := pbTestAllBuilder_()
	TextFormat_merge(strobj_(text), b)
	return b.build().(pbM_)
}

func pbValStr_(v any) string {
	switch x := v.(type) {
	case List:
		return pbListStr_(x)
	case *Descriptors_EnumValueDescriptor:
		return fmt.Sprintf("enum(%s/%d)", x.toString(), x.getNumber())
	}
	return pbListStr_(listOf_([]any{v}))
}

var pbTestAllTexts_ = [][2]string{
	{"empty", ``},
	{"singular", `f_double: 1.5 f_float: -2.25 f_int64: -9000000000 f_uint64: 18446744073709551615 f_int32: -7 f_fixed64: 18446744073709551615
		f_fixed32: 4294967295 f_bool: true f_string: "h\303\251" f_bytes: "\000\377x" f_uint32: 4294967295 f_enum: BIG f_sfixed32: -1 f_sfixed64: -1
		f_sint32: -65 f_sint64: -9000000000 f_big: 123456`},
	{"singular zeros", `f_double: 0 f_float: 0 f_int64: 0 f_uint64: 0 f_int32: 0 f_fixed64: 0 f_fixed32: 0 f_bool: false f_string: "" f_bytes: ""
		f_uint32: 0 f_enum: ZERO f_sfixed32: 0 f_sfixed64: 0 f_sint32: 0 f_sint64: 0 f_big: 0`},
	{"singular small", `f_double: 4.9e-324 f_float: 1.4e-45 f_int64: 1 f_uint64: 1 f_int32: 1 f_fixed64: 1 f_fixed32: 1 f_string: " " f_bytes: " "
		f_uint32: 1 f_enum: -2 f_sfixed32: 1 f_sfixed64: 1 f_sint32: 1 f_sint64: 1 f_big: -1`},
	{"floats", `f_double: -0.0 f_float: -0.0 o_double: nan o_float: nan r_double: [inf, -inf, nan, -0.0, 0] r_float: [inf, -inf, nan, -0.0, 0]`},
	{"optional zeros", `o_double: 0 o_float: 0 o_int64: 0 o_int32: 0 o_bool: false o_string: "" o_bytes: "" o_uint32: 0 o_enum: RED
		o_def_int32: 0 o_def_string: "" o_def_bool: false o_def_double: 0`},
	{"optional", `o_double: -1 o_float: 3.5 o_int64: 1 o_int32: -1 o_bool: true o_string: "s" o_bytes: "b" o_uint32: 7 o_enum: GREEN
		o_def_int32: 7 o_def_string: "dflt" o_def_bool: true o_def_double: 2.5`},
	{"packed", `r_double: [1, -2.5] r_float: [0.5, 0] r_int64: [0, -1, 9000000000] r_uint64: [0, 18446744073709551615]
		r_int32: [0, -1, 127, 128, 2147483647, -2147483648] r_fixed64: [1, 18446744073709551615] r_fixed32: [1, 4294967295] r_bool: [true, false, true]
		r_uint32: [0, 4294967295] r_enum: [ONE, ZERO, NEG, 77] r_sfixed32: [-1, 1] r_sfixed64: [-1, 1] r_sint32: [-1, 1, -2147483648]
		r_sint64: [-1, 1, -9223372036854775808]`},
	{"unpacked", `u_int32: 1 u_int32: -1 u_int32: [300, 0] r_string: "a" r_string: "" r_string: "c" r_bytes: "" r_bytes: "\001" u_enum: GREEN
		u_enum: RED u_enum: 2 r_msg { f_int32: 1 } r_msg { } r_msg { r_msg { f_string: "deep" } }`},
	{"oneof last wins", `c_int32: 5 c_string: "x"`},
	{"oneof int zero", `c_string: "x" c_int32: 0`},
	{"oneof bytes", `c_int32: 1 c_bytes: ""`},
	{"oneof msg", `c_bool: true c_msg { c_double: 0 }`},
	{"oneof double", `c_msg { } c_double: -0.5`},
	{"oneof bool", `c_double: 1 c_bool: false`},
	{"nested", `f_msg { f_msg { f_int32: 1 o_enum: GREEN } r_int32: [1, 2] } f_msg { f_msg { f_string: "merged" } r_int32: 3 }`},
}

// TestProtoTypedAllKinds: a hand-declared message with a field of every kind (singular,
// optional, with proto2 defaults, packed, unpacked, in a oneof): text format in, bytes out and
// back, the reflective view, the generic builder operations, merging, equality.
func TestProtoTypedAllKinds(t *testing.T) {
	r := &pbRec_{t: t}
	d := pbTestAll_desc_
	var msgs []pbM_
	for _, tx := range pbTestAllTexts_ {
		m := pbTestAllOf_(tx[1])
		msgs = append(msgs, m)
		r.msg(tx[0], m)
		all := m.(MessageOrBuilder).getAllFields()
		var parts []string
		for _, f := range d.sorted {
			if fd := f.descriptor_(); all.containsKey(fd) {
				parts = append(parts, f.name+"="+pbValStr_(all.get(fd)))
			}
		}
		r.p("%s fields: %s", tx[0], strings.Join(parts, " "))
	}
	// What every field reads as: in the empty message its default.
	for _, i := range []int{0, 1, 5, 6} {
		var parts []string
		for _, f := range d.sorted {
			parts = append(parts, f.name+"="+pbValStr_(pbGetField_(&msgs[i].pbm_().pbData_, f.descriptor_())))
		}
		r.p("%s reads: %s", pbTestAllTexts_[i][0], strings.Join(parts, " "))
	}
	var eq strings.Builder
	for _, a := range msgs {
		for _, b := range msgs {
			if a.pbm_().equals(b) {
				eq.WriteByte('1')
			} else {
				eq.WriteByte('0')
			}
		}
		eq.WriteByte(' ')
	}
	r.p("equals: %s", eq.String())

	// The generic builder operations on every field.
	b := msgs[1].pbm_().toBuilder().(pbB_).pbb_()
	b.mergeData_(pbDataOf_(msgs[6]))
	b.mergeData_(pbDataOf_(msgs[7]))
	b.mergeData_(pbDataOf_(msgs[8]))
	b.mergeData_(pbDataOf_(msgs[10]))
	r.msg("merged", b)
	for _, f := range d.fields {
		i := f.idx
		switch {
		case f.rep:
			n := b.cnt_(i)
			last := b.at_(i, n-1)
			b.addAt_(i, 0, last)
			b.setAt_(i, 1, b.at_(i, n))
			b.rm_(i, n-1)
			b.add_(i, b.at_(i, 0))
			r.p("%s: %d -> %d first %s", f.name, n, b.cnt_(i), pbValStr_(b.at_(i, 0)))
		case f.kind != pbMessage_:
			had, v := b.has_(i), b.get_(i)
			b.clr_(i)
			r.p("%s: %v %s -> %v %s", f.name, had, pbValStr_(v), b.has_(i), pbValStr_(b.get_(i)))
			if f.oneof == 0 && f.num%2 == 1 {
				b.set_(i, v)
			}
		}
	}
	r.p("case: %d", b.case_(1))
	r.msg("after the operations", b)
	for _, f := range d.fields {
		if f.rep {
			r.thrown(f.name+" index", func() { b.setAt_(f.idx, b.cnt_(f.idx), b.at_(f.idx, 0)) })
			b.clr_(f.idx)
		}
	}
	b.clrOneof_(1)
	r.msg("repeated cleared", b)
	r.msg("first still", msgs[1])
	for i, m := range msgs {
		for _, j := range []int{1, 6, 8, 12} {
			x := m.pbm_().toBuilder().(pbB_).pbb_()
			x.mergeData_(pbDataOf_(msgs[j]))
			r.msg(fmt.Sprintf("merge %d<-%d", i, j), x)
		}
	}

	// The wire format, read.
	for _, w := range [][2]string{
		{"unpacked input of a packed field", "e802 01 e802 ffffffff0f ea02 02 0506 fd02 01000000 fa02 08 0200000003000000 c902 000000000000f03f"},
		{"packed input of an unpacked field", "e203 03 01ac02 e003 07"},
		{"closed enum", "8802 63 8802 02 8004 01 8004 63 8004 02 8204 03 026301"},
		{"closed enum unknown only", "8802 63"},
		{"message twice", "52 02 2801 52 02 2802 52 03 4a0178 52 00"},
		{"oneof on the wire", "b804 05 c204 01 78 b804 00"},
		{"oneof message twice", "d204 02 2801 b804 01 d204 02 4a00 d204 03 4a0179"},
		{"big numbers and unknown", "807d 01 c03e 7b d005 01 c03e 7c"},
		{"varint widths", "28 ffffffffffffffffff01 60 ffffffffffffffffff01 68 ffffffffffffffffff01 40 02 18 8080808080808080807f"},
		{"bytes that are not utf8", "ea03 02 c328 5a 02 c328"},
	} {
		data := pbHex_(w[1])
		m := pbParse_(d, data, false).(pbM_)
		r.msg(w[0], m)
		if alias := pbParse_(d, data, true).(pbM_); !alias.pbm_().equals(m) {
			t.Errorf("%s: parsed differently with aliasing", w[0])
		}
	}
	for _, w := range [][2]string{
		{"truncated double", "09 0102"}, {"truncated fixed32", "3d 0102"}, {"truncated string", "4a 05 6162"}, {"bad utf8", "4a 02 c328"},
		{"truncated packed", "ea02 03 0506"}, {"truncated message", "52 03 2801"}, {"long varint", "28 ffffffffffffffffffff01"},
		{"negative length", "4a ffffffff0f"},
	} {
		r.thrown(w[0], func() { pbParse_(d, pbHex_(w[1]), false) })
	}
	r.check("allkinds")
}

func pbBS_(b []byte) *ByteString { return &ByteString{b_: b} }

// TestProtoTypedJavaFixtures reads every fixture protobuf-java wrote (manifest.tsv): parsing
// the bytes and writing them again gives the same bytes, the text formats and the set of fields
// present are Java's, and Java's text parses to the same bytes; parse_errors.tsv holds the
// text parser's messages.
func TestProtoTypedJavaFixtures(t *testing.T) {
	short := &TextFormat_Printer{singleLine_: true, escapeNonAscii_: true}
	for _, line := range pbLines_(t, "manifest.tsv") {
		name, d := line[0], pbDescs_[line[1]]
		if d == nil {
			t.Fatalf("%s: no message %s", name, line[1])
		}
		want := pbFixture_(t, name+".bin")
		input := want
		if name == "requirement_unknown" {
			input = pbFixture_(t, name+".input")
		}
		in := (&ByteString{b_: input}).newCodedInput()
		in.setRecursionLimit(math.MaxInt32)
		m := pbParseCoded_(d, in).(pbM_).pbm_()
		if got := m.bytes_(); !bytes.Equal(got, want) {
			t.Errorf("%s: bytes differ:\n got %x\nwant %x", name, got, want)
		}
		if got, want := m.toString(), string(pbFixture_(t, name+".txt")); got != want {
			t.Errorf("%s: text differs:\n got %s\nwant %s", name, got, want)
		}
		if got, want := short.printToString(m.me_().(MessageOrBuilder)), strings.TrimRight(string(pbFixture_(t, name+".short")), "\n"); got != want {
			t.Errorf("%s: short text differs:\n got %s\nwant %s", name, got, want)
		}
		all := m.me_().(MessageOrBuilder).getAllFields()
		fields := ""
		for _, f := range d.sorted {
			if all.containsKey(f.descriptor_()) {
				fields += f.name + ","
			}
		}
		if len(line) > 2 && fields != line[2] || len(line) <= 2 && fields != "" {
			t.Errorf("%s: fields present: %s, want %v", name, fields, line[2:])
		}
		// Through a builder and back, and Java's text parsed (unknown fields have no text to parse).
		if got := m.toBuilder().(pbB_).pbb_().build().(pbM_).pbm_(); !got.equals(m.me_()) || !bytes.Equal(got.bytes_(), want) {
			t.Errorf("%s: differs through its builder", name)
		}
		if name == "requirement_unknown" || name == "astnode_deep" {
			continue
		}
		b := d.newBuilder_().me_().(Message_Builder)
		TextFormat_merge(strobj_(string(pbFixture_(t, name+".txt"))), b)
		if got := b.build().(pbM_).pbm_().bytes_(); !bytes.Equal(got, want) {
			t.Errorf("%s: bytes of the parsed text differ:\n got %x\nwant %x", name, got, want)
		}
	}
	b := ConformanceConfig_newBuilder()
	TextFormat_merge(strobj_(string(pbFixture_(t, "conformance_parse.textproto"))), b)
	if got, want := b.build().(pbM_).pbm_().bytes_(), pbFixture_(t, "conformance_parse.bin"); !bytes.Equal(got, want) {
		t.Errorf("conformance_parse.textproto: bytes differ:\n got %x\nwant %x", got, want)
	}
	unescape := strings.NewReplacer(`\n`, "\n", `\t`, "\t", `\\`, `\`)
	for _, line := range pbLines_(t, "parse_errors.tsv") {
		_, msg := pbThrown_(func() { TextFormat_merge(strobj_(unescape.Replace(line[0])), ConformanceConfig_newBuilder()) })
		if want := unescape.Replace(line[1]); msg != want {
			t.Errorf("%s:\n got %q\nwant %q", line[0], msg, want)
		}
	}
}

// TestProtoTypedStorage: a message is one allocation, header and typed fields together, and
// building or parsing one boxes nothing.
func TestProtoTypedStorage(t *testing.T) {
	if n := unsafe.Sizeof(AstNode{}); n > 112 || n != unsafe.Sizeof(AstNode_Builder{}) {
		t.Errorf("AstNode is %d bytes, its builder %d", n, unsafe.Sizeof(AstNode_Builder{}))
	}
	var sink *AstNode
	build := testing.AllocsPerRun(100, func() {
		sink = AstNode_newBuilder().setKind(NodeKind_IDENTIFIER).setStringValuePointer(70000).setRelativeLine(-300).setRelativeColumn(1 << 20).
			setBooleanProperties(1 << 40).setType(1000).setSourceFile(5000).setDoubleValue(1e300).build().(*AstNode)
	})
	if build > 2 { // the builder and the message
		t.Errorf("building a leaf AstNode: %v allocations", build)
	}
	data := &ByteString{b_: sink.pbm_().bytes_()}
	parse := testing.AllocsPerRun(100, func() { sink = AstNode_parseFrom_ByteString(data) })
	if parse > 1 {
		t.Errorf("parsing a leaf AstNode: %v allocations", parse)
	}
	if sink.getDoubleValue() != 1e300 || sink.getBooleanProperties() != 1<<40 || sink.getType() != 1000 {
		t.Errorf("parsed %s", sink.toString())
	}
}
