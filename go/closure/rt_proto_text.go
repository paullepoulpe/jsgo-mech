package closure

// Protocol buffers text format: com.google.protobuf.TextFormat.

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

type TextFormat struct {
}

// ---------------------------------------------------------------- escaping

func pbEscapeBytes_(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		switch c {
		case 7:
			sb.WriteString("\\a")
		case '\b':
			sb.WriteString("\\b")
		case '\f':
			sb.WriteString("\\f")
		case '\n':
			sb.WriteString("\\n")
		case '\r':
			sb.WriteString("\\r")
		case '\t':
			sb.WriteString("\\t")
		case 11:
			sb.WriteString("\\v")
		case '\\':
			sb.WriteString("\\\\")
		case '\'':
			sb.WriteString("\\'")
		case '"':
			sb.WriteString("\\\"")
		default:
			if c >= 0x20 && c <= 0x7e {
				sb.WriteByte(c)
			} else {
				sb.WriteByte('\\')
				sb.WriteByte('0' + (c>>6)&3)
				sb.WriteByte('0' + (c>>3)&7)
				sb.WriteByte('0' + c&7)
			}
		}
	}
	return sb.String()
}

// TextFormat_InvalidEscapeSequenceException is TextFormat.InvalidEscapeSequenceException.
type TextFormat_InvalidEscapeSequenceException struct {
	IOException
}

func pbHexVal_(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// pbUnescape_ undoes pbEscapeBytes_ (and the other escapes text format allows).
func pbUnescape_(s string) ([]byte, string) {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' {
			out = append(out, c)
			continue
		}
		if i+1 >= len(s) {
			return nil, "Invalid escape sequence: '\\' at end of string."
		}
		i++
		c = s[i]
		switch {
		case c >= '0' && c <= '7':
			v := int(c - '0')
			if i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '7' {
				i++
				v = v*8 + int(s[i]-'0')
			}
			if i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '7' {
				i++
				v = v*8 + int(s[i]-'0')
			}
			out = append(out, byte(v))
		case c == 'a':
			out = append(out, 7)
		case c == 'b':
			out = append(out, '\b')
		case c == 'f':
			out = append(out, '\f')
		case c == 'n':
			out = append(out, '\n')
		case c == 'r':
			out = append(out, '\r')
		case c == 't':
			out = append(out, '\t')
		case c == 'v':
			out = append(out, 11)
		case c == '\\' || c == '\'' || c == '"' || c == '?':
			out = append(out, c)
		case c == 'x':
			if i+1 >= len(s) || pbHexVal_(s[i+1]) < 0 {
				return nil, "Invalid escape sequence: '\\x' with no digits"
			}
			i++
			v := pbHexVal_(s[i])
			if i+1 < len(s) && pbHexVal_(s[i+1]) >= 0 {
				i++
				v = v*16 + pbHexVal_(s[i])
			}
			out = append(out, byte(v))
		case c == 'u' || c == 'U':
			n := 4
			if c == 'U' {
				n = 8
			}
			if i+n >= len(s) {
				return nil, "Invalid escape sequence: '\\" + string(c) + "' with too few hex chars"
			}
			v := 0
			for j := 1; j <= n; j++ {
				h := pbHexVal_(s[i+j])
				if h < 0 {
					return nil, "Invalid escape sequence: '\\" + string(c) + "' with too few hex chars"
				}
				v = v*16 + h
			}
			if v > 0x10ffff {
				return nil, "Invalid escape sequence: '\\U" + s[i+1:i+1+n] + "' is not a valid code point value"
			}
			if v >= 0xd800 && v <= 0xdfff {
				return nil, "Invalid escape sequence: '\\" + string(c) + s[i+1:i+1+n] + "' refers to a surrogate code unit"
			}
			i += n
			out = utf8.AppendRune(out, rune(v))
		default:
			return nil, "Invalid escape sequence: '\\" + string(c) + "'"
		}
	}
	return out, ""
}

// ---------------------------------------------------------------- printing

type TextFormat_Printer struct {
	singleLine_     bool
	escapeNonAscii_ bool
}

var pbDefaultPrinter_ = &TextFormat_Printer{escapeNonAscii_: true}

func TextFormat_printer() *TextFormat_Printer { return pbDefaultPrinter_ }

func (this *TextFormat_Printer) escapingNonAscii(p0 bool) *TextFormat_Printer {
	return &TextFormat_Printer{singleLine_: this.singleLine_, escapeNonAscii_: p0}
}

func (this *TextFormat_Printer) emittingSingleLine(p0 bool) *TextFormat_Printer {
	return &TextFormat_Printer{singleLine_: p0, escapeNonAscii_: this.escapeNonAscii_}
}

func (this *TextFormat_Printer) printToString(p0 MessageOrBuilder) string {
	return pbText_(pbDataOf_(p0), this.singleLine_, this.escapeNonAscii_)
}

func (this *TextFormat_Printer) shortDebugString(p0 MessageOrBuilder) string {
	return pbText_(pbDataOf_(p0), true, this.escapeNonAscii_)
}

func (this *TextFormat_Printer) print(p0 MessageOrBuilder, p1 Appendable) {
	p1.(interface{ append(string) *StringBuilder }).append(this.printToString(p0))
}

type pbTextOut_ struct {
	sb     strings.Builder
	single bool
	ascii  bool
	indent int
	bol    bool
}

func (o *pbTextOut_) print_(s string) {
	if o.bol {
		o.bol = false
		if !o.single {
			for i := 0; i < o.indent; i++ {
				o.sb.WriteString("  ")
			}
		}
	}
	o.sb.WriteString(s)
}

func (o *pbTextOut_) eol_() {
	if o.single {
		o.sb.WriteByte(' ')
	} else {
		o.sb.WriteByte('\n')
	}
	o.bol = true
}

func pbText_(d *pbData_, single bool, ascii bool) string {
	o := &pbTextOut_{single: single, ascii: ascii, bol: true}
	o.message_(d)
	s := o.sb.String()
	if single {
		s = strings.TrimRight(s, " ")
	}
	return s
}

func (o *pbTextOut_) message_(d *pbData_) {
	for _, f := range d.d_.sorted {
		v := d.v_[f.idx]
		if v == nil {
			continue
		}
		if f.rep {
			for _, e := range v.(*pbRep_).a {
				o.field_(f, e)
			}
		} else {
			o.field_(f, v)
		}
	}
	o.unknown_(d.unk_)
}

func (o *pbTextOut_) escape_(s string) string {
	if o.ascii {
		return pbEscapeBytes_([]byte(s))
	}
	r := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\n", "\\n")
	return r.Replace(s)
}

func (o *pbTextOut_) field_(f *pbField_, v any) {
	o.print_(f.name)
	if f.kind == pbMessage_ {
		o.print_(" {")
		o.eol_()
		o.indent++
		o.message_(pbDataOf_(v))
		o.indent--
		o.print_("}")
		o.eol_()
		return
	}
	o.print_(": ")
	switch f.kind {
	case pbInt32_, pbSint32_, pbSfixed32_:
		o.print_(istr_(int64(v.(int32))))
	case pbUint32_, pbFixed32_:
		o.print_(istr_(int64(uint32(v.(int32)))))
	case pbInt64_, pbSint64_, pbSfixed64_:
		o.print_(istr_(v.(int64)))
	case pbUint64_, pbFixed64_:
		o.print_(strconv.FormatUint(uint64(v.(int64)), 10))
	case pbBool_:
		o.print_(boolstr_(v.(bool)))
	case pbFloat_:
		o.print_(Float_toString(v.(float32)))
	case pbDouble_:
		o.print_(Double_toString(v.(float64)))
	case pbString_:
		o.print_("\"" + o.escape_(v.(string)) + "\"")
	case pbBytes_:
		o.print_("\"" + pbEscapeBytes_(v.(*ByteString).b_) + "\"")
	case pbEnum_:
		if name, ok := f.ed.nameOf_(v.(int32)); ok {
			o.print_(name)
		} else {
			o.print_(istr_(int64(v.(int32))))
		}
	}
	o.eol_()
}

// pbValidUnknown_ reports whether raw parses as a sequence of fields.
func pbValidUnknown_(raw []byte) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
		}
	}()
	for pos := 0; pos < len(raw); {
		var tag uint64
		tag, pos = pbVarint_(raw, pos)
		if tag>>3 == 0 || tag&7 == 4 {
			return false
		}
		pos = pbSkip_(raw, pos, int(tag&7), int32(tag>>3))
	}
	return true
}

func (o *pbTextOut_) unknown_(raw []byte) {
	for pos := 0; pos < len(raw); {
		var tag uint64
		tag, pos = pbVarint_(raw, pos)
		num := istr_(int64(tag >> 3))
		switch tag & 7 {
		case 0:
			var v uint64
			v, pos = pbVarint_(raw, pos)
			o.print_(num + ": " + strconv.FormatUint(v, 10))
			o.eol_()
		case 5:
			v := uint32(raw[pos]) | uint32(raw[pos+1])<<8 | uint32(raw[pos+2])<<16 | uint32(raw[pos+3])<<24
			pos += 4
			h := strconv.FormatUint(uint64(v), 16)
			o.print_(num + ": 0x" + strings.Repeat("0", 8-len(h)) + h)
			o.eol_()
		case 1:
			var v uint64
			for i := 7; i >= 0; i-- {
				v = v<<8 | uint64(raw[pos+i])
			}
			pos += 8
			h := strconv.FormatUint(v, 16)
			o.print_(num + ": 0x" + strings.Repeat("0", 16-len(h)) + h)
			o.eol_()
		case 2:
			var n int
			n, pos = pbLen_(raw, pos)
			sub := raw[pos : pos+n]
			pos += n
			if pbValidUnknown_(sub) {
				o.print_(num + ": {")
				o.eol_()
				o.indent++
				o.unknown_(pbSortUnknown_(sub))
				o.indent--
				o.print_("}")
			} else {
				o.print_(num + ": \"" + pbEscapeBytes_(sub) + "\"")
			}
			o.eol_()
		case 3:
			end := pbSkip_(raw, pos, 3, int32(tag>>3))
			// The group's content excludes its end tag.
			inner := raw[pos:end]
			for cut := pos; cut < end; {
				var t uint64
				at := cut
				t, cut = pbVarint_(raw, cut)
				if t&7 == 4 && t>>3 == tag>>3 {
					inner = raw[pos:at]
					break
				}
				cut = pbSkip_(raw, cut, int(t&7), int32(t>>3))
			}
			pos = end
			o.print_(num + ": {")
			o.eol_()
			o.indent++
			o.unknown_(pbSortUnknown_(inner))
			o.indent--
			o.print_("}")
			o.eol_()
		default:
			return
		}
	}
}

// ---------------------------------------------------------------- parsing

// TextFormat_ParseException is TextFormat.ParseException.
type TextFormat_ParseException struct {
	IOException
	line_   int32
	column_ int32
}

func init() {
	javaNames_["TextFormat_ParseException"] = "com.google.protobuf.TextFormat$ParseException"
	javaNames_["TextFormat_InvalidEscapeSequenceException"] = "com.google.protobuf.TextFormat$InvalidEscapeSequenceException"
	javaNames_["TextFormat_Parser"] = "com.google.protobuf.TextFormat$Parser"
	javaNames_["CodedOutputStream"] = "com.google.protobuf.CodedOutputStream"
	javaNames_["Descriptors_Descriptor"] = "com.google.protobuf.Descriptors$Descriptor"
	javaNames_["Descriptors_EnumValueDescriptor"] = "com.google.protobuf.Descriptors$EnumValueDescriptor"
}

func pbParseError_(line, col int, msg string) {
	e := &TextFormat_ParseException{line_: int32(line), column_: int32(col)}
	e.self_ = e
	e.init_IOException_String(strconv.Itoa(line) + ":" + strconv.Itoa(col) + ": " + msg)
	panic(e.self())
}

type TextFormat_Parser struct {
}

var pbDefaultParser_ = &TextFormat_Parser{}

// merge is TextFormat.Parser.merge(CharSequence, Message.Builder).
func (this *TextFormat_Parser) merge(p0 any, p1 Message_Builder) {
	if p0 == nil || p1 == nil {
		pbNPE_()
	}
	t := &pbTok_{text: objstr_(p0)}
	t.skipSpace_()
	t.next_()
	var unknown []string
	b := p1.(pbB_).pbb_()
	for !t.atEnd_() {
		t.mergeField_(b, &unknown)
	}
	if len(unknown) > 0 {
		parts := strings.Split(unknown[0], ":")
		line, _ := strconv.Atoi(parts[0])
		col, _ := strconv.Atoi(parts[1])
		pbParseError_(line, col, "Input contains unknown fields and/or extensions:\n"+strings.Join(unknown, "\n"))
	}
}

// TextFormat_merge is TextFormat.merge(CharSequence, Message.Builder).
func TextFormat_merge(p0 any, p1 Message_Builder) { pbDefaultParser_.merge(p0, p1) }

type pbTok_ struct {
	text      string
	pos       int
	cur       string
	line, col int // of the character at pos
	tokLine   int
	tokCol    int
	prevLine  int
	prevCol   int
}

func (t *pbTok_) advance_(n int) {
	for i := 0; i < n; i++ {
		if t.text[t.pos] == '\n' {
			t.line++
			t.col = 0
		} else {
			t.col++
		}
		t.pos++
	}
}

func (t *pbTok_) skipSpace_() {
	for t.pos < len(t.text) {
		c := t.text[t.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == 11 {
			t.advance_(1)
		} else if c == '#' {
			for t.pos < len(t.text) && t.text[t.pos] != '\n' {
				t.advance_(1)
			}
		} else {
			return
		}
	}
}

func pbIsLetter_(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
}

func pbIsDigit_(c byte) bool { return c >= '0' && c <= '9' }

func (t *pbTok_) next_() {
	t.prevLine, t.prevCol = t.tokLine, t.tokCol
	t.tokLine, t.tokCol = t.line, t.col
	if t.pos >= len(t.text) {
		t.cur = ""
		return
	}
	s := t.text
	i := t.pos
	c := s[i]
	end := i + 1
	word := func(c byte) bool { return pbIsLetter_(c) || pbIsDigit_(c) || c == '+' || c == '-' }
	switch {
	case pbIsLetter_(c):
		for end < len(s) && word(s[end]) {
			end++
		}
	case pbIsDigit_(c) || c == '+' || c == '-' || c == '.' && i+1 < len(s) && (pbIsDigit_(s[i+1]) || s[i+1] == '+' || s[i+1] == '-'):
		if c == '.' {
			end++
		}
		for end < len(s) && (word(s[end]) || s[end] == '.') {
			end++
		}
	case c == '"' || c == '\'':
		for end < len(s) && s[end] != c && s[end] != '\n' {
			if s[end] == '\\' && end+1 < len(s) {
				end++
			}
			end++
		}
		if end < len(s) && s[end] == c {
			end++
		}
	default:
		// A single character; keep a whole UTF-8 sequence together.
		_, n := utf8.DecodeRuneInString(s[i:])
		end = i + n
	}
	t.cur = s[i:end]
	t.advance_(end - i)
	t.skipSpace_()
}

func (t *pbTok_) atEnd_() bool { return t.cur == "" }

func (t *pbTok_) fail_(msg string) { pbParseError_(t.tokLine+1, t.tokCol+1, msg) }

func (t *pbTok_) failPrev_(msg string) { pbParseError_(t.prevLine+1, t.prevCol+1, msg) }

func (t *pbTok_) tryConsume_(tok string) bool {
	if t.cur == tok {
		t.next_()
		return true
	}
	return false
}

func (t *pbTok_) consume_(tok string) {
	if !t.tryConsume_(tok) {
		t.fail_("Expected \"" + tok + "\".")
	}
}

func (t *pbTok_) lookingAtInteger_() bool {
	if t.cur == "" {
		return false
	}
	c := t.cur[0]
	return pbIsDigit_(c) || c == '-' || c == '+'
}

func (t *pbTok_) consumeIdentifier_() string {
	for i := 0; i < len(t.cur); i++ {
		c := t.cur[i]
		if !(pbIsLetter_(c) || pbIsDigit_(c) || c == '.') {
			t.fail_("Expected identifier. Found '" + t.cur + "'")
		}
	}
	if t.cur == "" {
		t.fail_("Expected identifier. Found ''")
	}
	s := t.cur
	t.next_()
	return s
}

// pbParseInteger_ is TextFormat.parseInteger: decimal, hex (0x) or octal (0).
func pbParseInteger_(text string, signed bool, long bool) (int64, string) {
	s := text
	neg := false
	if strings.HasPrefix(s, "-") {
		if !signed {
			return 0, "Number must be positive: " + text
		}
		neg = true
		s = s[1:]
	}
	radix := 10
	if strings.HasPrefix(s, "0x") {
		s = s[2:]
		radix = 16
	} else if strings.HasPrefix(s, "0") {
		radix = 8
	}
	if s == "" || s[0] == '+' || s[0] == '-' {
		return 0, "For input string: \"" + s + "\""
	}
	u, err := strconv.ParseUint(s, radix, 64)
	if err != nil {
		if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
			return 0, pbRangeMsg_(text, signed, long)
		}
		return 0, "For input string: \"" + s + "\""
	}
	bits := uint(32)
	if long {
		bits = 64
	}
	if signed {
		max := uint64(1) << (bits - 1)
		if neg && u > max || !neg && u > max-1 {
			return 0, pbRangeMsg_(text, signed, long)
		}
		if neg {
			return -int64(u), ""
		}
		return int64(u), ""
	}
	if !long && u > math.MaxUint32 {
		return 0, pbRangeMsg_(text, signed, long)
	}
	return int64(u), ""
}

func pbRangeMsg_(text string, signed bool, long bool) string {
	msg := "Number out of range for "
	if long {
		msg += "64-bit "
	} else {
		msg += "32-bit "
	}
	if signed {
		msg += "signed"
	} else {
		msg += "unsigned"
	}
	return msg + " integer: " + text
}

func (t *pbTok_) consumeInteger_(signed bool, long bool) int64 {
	v, err := pbParseInteger_(t.cur, signed, long)
	if err != "" {
		t.fail_("Couldn't parse integer: " + err)
	}
	t.next_()
	return v
}

func (t *pbTok_) consumeFloat_(bits int) float64 {
	tok := t.cur
	low := strings.ToLower(tok)
	var v float64
	switch {
	case low == "inf" || low == "infinity" || bits == 32 && (low == "inff" || low == "infinityf"):
		v = math.Inf(1)
	case low == "-inf" || low == "-infinity" || bits == 32 && (low == "-inff" || low == "-infinityf"):
		v = math.Inf(-1)
	case low == "nan" || low == "nanf":
		v = math.NaN()
	default:
		num := tok
		if n := len(num); n > 1 && (num[n-1] == 'f' || num[n-1] == 'F' || num[n-1] == 'd' || num[n-1] == 'D') && !strings.HasPrefix(low, "0x") {
			num = num[:n-1]
		}
		var err error
		v, err = strconv.ParseFloat(num, bits)
		if err != nil {
			if ne, ok := err.(*strconv.NumError); !ok || ne.Err != strconv.ErrRange {
				t.fail_("Couldn't parse number: For input string: \"" + tok + "\"")
			}
		}
	}
	t.next_()
	return v
}

func (t *pbTok_) consumeBool_() bool {
	switch t.cur {
	case "true", "True", "t", "1":
		t.next_()
		return true
	case "false", "False", "f", "0":
		t.next_()
		return false
	}
	t.fail_("Expected \"true\" or \"false\". Found \"" + t.cur + "\".")
	return false
}

func (t *pbTok_) consumeBytes_() []byte {
	var out []byte
	first := true
	for first || strings.HasPrefix(t.cur, "\"") || strings.HasPrefix(t.cur, "'") {
		first = false
		if t.cur == "" || t.cur[0] != '"' && t.cur[0] != '\'' {
			t.fail_("Expected string.")
		}
		q := t.cur[0]
		if len(t.cur) < 2 || t.cur[len(t.cur)-1] != q {
			t.fail_("String missing ending quote.")
		}
		b, err := pbUnescape_(t.cur[1 : len(t.cur)-1])
		if err != "" {
			t.fail_(err)
		}
		out = append(out, b...)
		t.next_()
	}
	return out
}

func (t *pbTok_) mergeField_(b *AbstractMessageLite_Builder, unknown *[]string) {
	d := b.d_
	var f *pbField_
	if t.tryConsume_("[") {
		name := t.consumeIdentifier_()
		for t.tryConsume_(".") {
			name += "." + t.consumeIdentifier_()
		}
		*unknown = append(*unknown, strconv.Itoa(t.prevLine+1)+":"+strconv.Itoa(t.prevCol+1)+":\t"+d.name+".["+name+"]")
		t.consume_("]")
	} else {
		name := t.consumeIdentifier_()
		f = d.byName[name]
		if f == nil {
			*unknown = append(*unknown, strconv.Itoa(t.prevLine+1)+":"+strconv.Itoa(t.prevCol+1)+":\t"+d.name+"."+name)
		}
	}
	if f == nil {
		if t.tryConsume_(":") && t.cur != "{" && t.cur != "<" {
			t.skipValue_()
		} else {
			t.skipMessage_()
		}
		return
	}
	if f.kind == pbMessage_ {
		t.tryConsume_(":")
	} else {
		t.consume_(":")
	}
	if f.rep && t.tryConsume_("[") {
		if !t.tryConsume_("]") {
			for {
				t.value_(b, f, unknown)
				if t.tryConsume_("]") {
					break
				}
				t.consume_(",")
			}
		}
	} else {
		t.value_(b, f, unknown)
	}
	if !t.tryConsume_(";") {
		t.tryConsume_(",")
	}
}

func (t *pbTok_) value_(b *AbstractMessageLite_Builder, f *pbField_, unknown *[]string) {
	var v any
	switch f.kind {
	case pbMessage_:
		end := "}"
		if t.tryConsume_("<") {
			end = ">"
		} else {
			t.consume_("{")
		}
		var sub any
		if f.rep {
			sub = b.addBld_(f.idx)
		} else {
			sub = b.bld_(f.idx)
		}
		sb := sub.(pbB_).pbb_()
		for !t.tryConsume_(end) {
			if t.atEnd_() {
				t.fail_("Expected \"" + end + "\".")
			}
			t.mergeField_(sb, unknown)
		}
		return
	case pbInt32_, pbSint32_, pbSfixed32_:
		v = int32(t.consumeInteger_(true, false))
	case pbUint32_, pbFixed32_:
		v = int32(uint32(t.consumeInteger_(false, false)))
	case pbInt64_, pbSint64_, pbSfixed64_:
		v = t.consumeInteger_(true, true)
	case pbUint64_, pbFixed64_:
		v = t.consumeInteger_(false, true)
	case pbBool_:
		v = t.consumeBool_()
	case pbFloat_:
		v = float32(t.consumeFloat_(32))
	case pbDouble_:
		v = t.consumeFloat_(64)
	case pbString_:
		v = string(t.consumeBytes_())
	case pbBytes_:
		v = &ByteString{b_: t.consumeBytes_()}
	case pbEnum_:
		if t.lookingAtInteger_() {
			n := int32(t.consumeInteger_(true, false))
			if _, ok := f.ed.byNum[n]; !ok && f.ed.closed {
				t.failPrev_("Enum type \"" + f.ed.name + "\" has no value with number " + istr_(int64(n)) + ".")
			}
			v = n
		} else {
			id := t.consumeIdentifier_()
			n, ok := f.ed.byName[id]
			if !ok {
				t.failPrev_("Enum type \"" + f.ed.name + "\" has no value named \"" + id + "\".")
			}
			v = n
		}
	}
	if f.rep {
		b.append_(f, v)
	} else {
		b.put_(f, v)
	}
}

func (t *pbTok_) skipMessage_() {
	end := "}"
	if t.tryConsume_("<") {
		end = ">"
	} else {
		t.consume_("{")
	}
	for t.cur != ">" && t.cur != "}" {
		if t.atEnd_() {
			t.fail_("Expected \"" + end + "\".")
		}
		t.skipField_()
	}
	t.consume_(end)
}

func (t *pbTok_) skipField_() {
	if t.tryConsume_("[") {
		for {
			t.consumeIdentifier_()
			if !t.tryConsume_(".") {
				break
			}
		}
		t.consume_("]")
	} else {
		t.consumeIdentifier_()
	}
	if t.tryConsume_(":") && t.cur != "<" && t.cur != "{" {
		t.skipValue_()
	} else {
		t.skipMessage_()
	}
	if !t.tryConsume_(";") {
		t.tryConsume_(",")
	}
}

func (t *pbTok_) skipOne_() {
	if strings.HasPrefix(t.cur, "\"") || strings.HasPrefix(t.cur, "'") {
		t.consumeBytes_()
		return
	}
	if t.atEnd_() || t.cur == "}" || t.cur == ">" || t.cur == "]" || t.cur == ":" || t.cur == "," || t.cur == ";" {
		t.fail_("Invalid field value: " + t.cur)
	}
	t.next_()
}

func (t *pbTok_) skipValue_() {
	if t.tryConsume_("[") {
		if t.tryConsume_("]") {
			return
		}
		for {
			if t.cur == "{" || t.cur == "<" {
				t.skipMessage_()
			} else {
				t.skipOne_()
			}
			if t.tryConsume_("]") {
				return
			}
			t.consume_(",")
		}
	}
	t.skipOne_()
}

func (this *TextFormat_ParseException) up_IOException() *IOException {
	if this == nil {
		return nil
	}
	return &this.IOException
}

func (this *TextFormat_ParseException) getLine() int32 { return this.line_ }

func (this *TextFormat_ParseException) getColumn() int32 { return this.column_ }
