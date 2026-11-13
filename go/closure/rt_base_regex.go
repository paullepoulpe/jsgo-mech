package closure

// java.util.regex on Go's regexp package.
//
// A Java pattern is translated to RE2 syntax (see translateRegex_). Constructs
// RE2 cannot express (backreferences, lookaround, possessive quantifiers,
// atomic groups, class intersection) raise PatternSyntaxException. Matching is
// done on the UTF-8 bytes; every index a Matcher takes or returns is converted
// from/to UTF-16 units.

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	reUnixLines_       = 1
	reCaseInsensitive_ = 2
	reComments_        = 4
	reMultiline_       = 8
	reLiteral_         = 16
	reDotall_          = 32
)

type Pattern struct {
	src      string
	flags_   int32
	goSrc    string
	re       *regexp.Regexp
	reCtx    *regexp.Regexp // one character of left context, then the pattern
	reAt     *regexp.Regexp // anchored at the start
	reFull   *regexp.Regexp // anchored at both ends
	groupMap []int          // Java group number -> Go group number
	eol      []int          // Go group numbers of the groups that stand for `$`
	names    map[string]int
}

func newPatternSyntaxException_(desc string, regex string, index int) *PatternSyntaxException {
	msg := desc
	if index >= 0 {
		msg += " near index " + strconv.Itoa(index)
	}
	msg += "\n" + regex
	this := &PatternSyntaxException{}
	this.self_ = this
	this.init_IllegalArgumentException_String(msg)
	return this
}

type reTranslator_ struct {
	p       string
	i       int
	out     strings.Builder
	flags   int32
	nGo     int
	pat     *Pattern
	errDesc string
	errIdx  int
}

func (t *reTranslator_) fail(desc string, idx int) {
	if t.errDesc == "" {
		t.errDesc, t.errIdx = desc, idx
	}
	t.i = len(t.p)
}

const reLineTerm_ = `(?:\r\n|[\n\r\x{85}\x{2028}\x{2029}])`

var rePosix_ = map[string]string{
	"Lower": "lower", "Upper": "upper", "ASCII": "ascii", "Alpha": "alpha", "Digit": "digit",
	"Alnum": "alnum", "Punct": "punct", "Graph": "graph", "Print": "print", "Blank": "blank",
	"Cntrl": "cntrl", "XDigit": "xdigit", "Space": "space",
}

var reJavaProps_ = map[string]string{
	"javaLowerCase": `\p{Ll}`, "javaUpperCase": `\p{Lu}`, "javaTitleCase": `\p{Lt}`,
	"javaDigit": `\p{Nd}`, "javaLetter": `\p{L}`, "javaLetterOrDigit": `\p{L}\p{Nd}`,
	"javaAlphabetic": `\p{L}\p{Nl}`, "javaWhitespace": `\t-\r\x{1c}-\x{1f} \x{1680}\x{2000}-\x{2006}\x{2008}-\x{200a}\x{2028}\x{2029}\x{205f}\x{3000}`,
	"IsAlphabetic": `\p{L}\p{Nl}`, "IsLetter": `\p{L}`, "IsDigit": `\p{Nd}`,
	"IsUppercase": `\p{Lu}`, "IsLowercase": `\p{Ll}`, "IsPunctuation": `\p{P}`,
	"IsWhite_Space": `\t-\r \x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`,
	"IsWhiteSpace":  `\t-\r \x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`,
	"L1":            `\x00-\x{ff}`,
}

// property translates \p{name} / \P{name}; inClass says the output goes inside [...].
func (t *reTranslator_) property(name string, neg bool, inClass bool, at int) {
	if strings.HasPrefix(name, "^") {
		neg = !neg
		name = name[1:]
	}
	if g, ok := rePosix_[name]; ok {
		s := "[:" + g + ":]"
		if neg {
			s = "[:^" + g + ":]"
		}
		if !inClass {
			s = "[" + s + "]"
		}
		t.out.WriteString(s)
		return
	}
	if body, ok := reJavaProps_[name]; ok {
		switch {
		case !inClass && neg:
			t.out.WriteString("[^" + body + "]")
		case !inClass:
			t.out.WriteString("[" + body + "]")
		case neg:
			t.fail("Negated property \\P{"+name+"} is not supported inside a character class", at)
		default:
			t.out.WriteString(body)
		}
		return
	}
	n := name
	for _, pre := range []string{"general_category=", "gc=", "script=", "sc=", "Is"} {
		if strings.HasPrefix(n, pre) {
			n = n[len(pre):]
			break
		}
	}
	_, isCat := unicode.Categories[n]
	_, isScript := unicode.Scripts[n]
	if !isCat && !isScript {
		// Script names are case-insensitive in Java.
		for k := range unicode.Scripts {
			if strings.EqualFold(k, n) {
				n, isScript = k, true
				break
			}
		}
	}
	if !isCat && !isScript {
		t.fail("Unknown character property name {"+name+"}", at)
		return
	}
	if neg {
		t.out.WriteString(`\P{` + n + `}`)
	} else {
		t.out.WriteString(`\p{` + n + `}`)
	}
}

func reHex_(cp rune) string { return `\x{` + strconv.FormatInt(int64(cp), 16) + `}` }

func isHexDigit_(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func (t *reTranslator_) literal(r rune, inClass bool) {
	if inClass {
		switch r {
		case '\\', ']', '[', '^', '-':
			t.out.WriteByte('\\')
		}
		t.out.WriteRune(r)
		return
	}
	t.out.WriteString(regexp.QuoteMeta(string(r)))
}

// escape translates the escape sequence at t.i (which is at the backslash).
func (t *reTranslator_) escape(inClass bool) {
	p := t.p
	at := t.i
	if at+1 >= len(p) {
		t.fail("Unexpected internal error", at)
		return
	}
	c := p[at+1]
	t.i = at + 2
	switch c {
	case 'Q':
		end := strings.Index(p[t.i:], `\E`)
		lit := p[t.i:]
		if end >= 0 {
			lit = p[t.i : t.i+end]
			t.i += end + 2
		} else {
			t.i = len(p)
		}
		for _, r := range lit {
			t.literal(r, inClass)
		}
	case 'E':
		// A stray \E is ignored.
	case 'u':
		if at+6 > len(p) || !isHexDigit_(p[at+2]) || !isHexDigit_(p[at+3]) || !isHexDigit_(p[at+4]) || !isHexDigit_(p[at+5]) {
			t.fail("Illegal Unicode escape sequence", at)
			return
		}
		v, _ := strconv.ParseInt(p[at+2:at+6], 16, 32)
		t.i = at + 6
		if v >= 0xD800 && v < 0xDC00 && t.i+6 <= len(p) && p[t.i] == '\\' && p[t.i+1] == 'u' {
			if lo, err := strconv.ParseInt(p[t.i+2:t.i+6], 16, 32); err == nil && lo >= 0xDC00 && lo < 0xE000 {
				v = 0x10000 + (v-0xD800)<<10 + (lo - 0xDC00)
				t.i += 6
			}
		}
		t.out.WriteString(reHex_(rune(v)))
	case 'x':
		if t.i < len(p) && p[t.i] == '{' {
			end := strings.IndexByte(p[t.i:], '}')
			if end < 0 {
				t.fail("Unclosed hexadecimal escape sequence", at)
				return
			}
			v, err := strconv.ParseInt(p[t.i+1:t.i+end], 16, 32)
			if err != nil || v > 0x10FFFF {
				t.fail("Illegal hexadecimal escape sequence", at)
				return
			}
			t.i += end + 1
			t.out.WriteString(reHex_(rune(v)))
			return
		}
		if t.i+2 > len(p) || !isHexDigit_(p[t.i]) || !isHexDigit_(p[t.i+1]) {
			t.fail("Illegal hexadecimal escape sequence", at)
			return
		}
		v, _ := strconv.ParseInt(p[t.i:t.i+2], 16, 32)
		t.i += 2
		t.out.WriteString(reHex_(rune(v)))
	case '0':
		v, n := 0, 0
		for n < 3 && t.i < len(p) && p[t.i] >= '0' && p[t.i] <= '7' {
			if n == 2 && v > 037 {
				break
			}
			v = v*8 + int(p[t.i]-'0')
			t.i++
			n++
		}
		if n == 0 {
			t.fail("Illegal octal escape sequence", at)
			return
		}
		t.out.WriteString(reHex_(rune(v)))
	case '1', '2', '3', '4', '5', '6', '7', '8', '9', 'k':
		t.fail("Backreferences are not supported", at)
	case 's', 'S':
		s := "[:space:]"
		if c == 'S' {
			s = "[:^space:]"
		}
		if !inClass {
			s = "[" + s + "]"
		}
		t.out.WriteString(s)
	case 'h', 'H', 'v', 'V':
		body := ` \t\x{a0}\x{1680}\x{180e}\x{2000}-\x{200a}\x{202f}\x{205f}\x{3000}`
		if c == 'v' || c == 'V' {
			body = `\n\x0B\f\r\x{85}\x{2028}\x{2029}`
		}
		neg := c == 'H' || c == 'V'
		switch {
		case inClass && neg:
			t.fail("Negated class escape is not supported inside a character class", at)
		case inClass:
			t.out.WriteString(body)
		case neg:
			t.out.WriteString("[^" + body + "]")
		default:
			t.out.WriteString("[" + body + "]")
		}
	case 'R':
		if inClass {
			t.fail("Illegal/unsupported escape sequence", at)
			return
		}
		t.out.WriteString(`(?:\r\n|[\n\x0B\f\r\x{85}\x{2028}\x{2029}])`)
	case 'd', 'D', 'w', 'W':
		t.out.WriteByte('\\')
		t.out.WriteByte(c)
	case 'b', 'B', 'A', 'z':
		if inClass {
			t.fail("Illegal/unsupported escape sequence", at)
			return
		}
		t.out.WriteByte('\\')
		t.out.WriteByte(c)
	case 'Z':
		if inClass {
			t.fail("Illegal/unsupported escape sequence", at)
			return
		}
		t.eolGroup()
	case 'G', 'X', 'N':
		t.fail("Illegal/unsupported escape sequence", at)
	case 't', 'n', 'r', 'f', 'a':
		t.out.WriteByte('\\')
		t.out.WriteByte(c)
	case 'e':
		t.out.WriteString(`\x1B`)
	case 'c':
		if t.i >= len(p) {
			t.fail("Illegal control escape sequence", at)
			return
		}
		t.out.WriteString(reHex_(rune(p[t.i] ^ 64)))
		t.i++
	case 'p', 'P':
		if t.i >= len(p) {
			t.fail("Illegal character property", at)
			return
		}
		var name string
		if p[t.i] == '{' {
			end := strings.IndexByte(p[t.i:], '}')
			if end < 0 {
				t.fail("Unclosed character family", at)
				return
			}
			name = p[t.i+1 : t.i+end]
			t.i += end + 1
		} else {
			name = p[t.i : t.i+1]
			t.i++
		}
		t.property(name, c == 'P', inClass, at)
	default:
		if c < 0x80 {
			if c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
				t.fail("Illegal/unsupported escape sequence", at)
				return
			}
			t.literal(rune(c), inClass)
			return
		}
		r, n := utf8.DecodeRuneInString(p[at+1:])
		t.i = at + 1 + n
		t.literal(r, inClass)
	}
}

func (t *reTranslator_) eolGroup() {
	if t.flags&reMultiline_ != 0 {
		t.out.WriteString("$")
		return
	}
	t.nGo++
	t.pat.eol = append(t.pat.eol, t.nGo)
	if t.flags&reUnixLines_ != 0 {
		t.out.WriteString(`(\n?\z)`)
	} else {
		t.out.WriteString("(" + reLineTerm_ + `?\z)`)
	}
}

// class translates a character class; t.i is at the opening bracket. When
// nested, only the members are written (a union with the enclosing class).
func (t *reTranslator_) class(nested bool) {
	p := t.p
	start := t.i
	t.i++
	neg := false
	if t.i < len(p) && p[t.i] == '^' {
		neg = true
		t.i++
	}
	if nested && neg {
		t.fail("Negated nested character classes are not supported", start)
		return
	}
	if !nested {
		t.out.WriteByte('[')
		if neg {
			t.out.WriteByte('^')
		}
	}
	first := true
	for {
		if t.i >= len(p) {
			t.fail("Unclosed character class", len(p)-1)
			return
		}
		c := p[t.i]
		switch {
		case c == ']' && !first:
			t.i++
			if !nested {
				t.out.WriteByte(']')
			}
			return
		case c == ']':
			t.out.WriteString(`\]`)
			t.i++
		case c == '\\':
			t.escape(true)
		case c == '[':
			t.class(true)
		case c == '&' && t.i+1 < len(p) && p[t.i+1] == '&':
			t.fail("Character class intersection is not supported", t.i)
			return
		case c == '^':
			t.out.WriteString(`\^`)
			t.i++
		default:
			_, n := utf8.DecodeRuneInString(p[t.i:])
			t.out.WriteString(p[t.i : t.i+n])
			t.i += n
		}
		first = false
	}
}

func (t *reTranslator_) group() {
	p := t.p
	at := t.i
	if at+1 < len(p) && p[at+1] == '?' {
		if at+2 >= len(p) {
			t.fail("Unknown inline modifier", at)
			return
		}
		c := p[at+2]
		if c == '<' && at+3 < len(p) && p[at+3] != '=' && p[at+3] != '!' {
			end := strings.IndexByte(p[at:], '>')
			if end < 0 {
				t.fail("named capturing group is missing trailing '>'", at)
				return
			}
			name := p[at+3 : at+end]
			if _, dup := t.pat.names[name]; dup {
				t.fail("Named capturing group <"+name+"> is already defined", at)
				return
			}
			t.nGo++
			t.pat.groupMap = append(t.pat.groupMap, t.nGo)
			t.pat.names[name] = len(t.pat.groupMap) - 1
			t.out.WriteString("(?P<" + name + ">")
			t.i = at + end + 1
			return
		}
		if c == '=' || c == '!' || c == '<' {
			t.fail("Lookaround is not supported", at)
			return
		}
		if c == '>' {
			t.fail("Atomic groups are not supported", at)
			return
		}
		// Inline flags: (?idmsux-idmsux) or (?idmsux-idmsux:X)
		j := at + 2
		var on, off strings.Builder
		cur := &on
		for j < len(p) && p[j] != ':' && p[j] != ')' {
			switch p[j] {
			case '-':
				cur = &off
			case 'i', 'm':
				cur.WriteByte(p[j])
			case 's':
				cur.WriteByte('s')
				if cur == &on {
					t.flags |= reDotall_
				} else {
					t.flags &^= reDotall_
				}
			case 'd', 'u', 'U':
			default:
				t.fail("Unknown inline modifier", j)
				return
			}
			j++
		}
		if j >= len(p) {
			t.fail("Unknown inline modifier", at)
			return
		}
		t.out.WriteString("(?" + on.String())
		if off.Len() > 0 {
			t.out.WriteString("-" + off.String())
		}
		t.out.WriteByte(p[j])
		t.i = j + 1
		return
	}
	t.nGo++
	t.pat.groupMap = append(t.pat.groupMap, t.nGo)
	t.out.WriteByte('(')
	t.i++
}

func translateRegex_(pat *Pattern) *PatternSyntaxException {
	p := pat.src
	t := &reTranslator_{p: p, flags: pat.flags_, pat: pat}
	pat.groupMap = []int{0}
	pat.names = map[string]int{}
	if pat.flags_&reComments_ != 0 {
		return newPatternSyntaxException_("The COMMENTS flag is not supported", p, -1)
	}
	prefix := ""
	if pat.flags_&reCaseInsensitive_ != 0 {
		prefix += "i"
	}
	if pat.flags_&reMultiline_ != 0 {
		prefix += "m"
	}
	if prefix != "" {
		t.out.WriteString("(?" + prefix + ")")
	}
	if pat.flags_&reLiteral_ != 0 {
		t.out.WriteString(regexp.QuoteMeta(p))
		t.i = len(p)
	}
	for t.i < len(p) {
		c := p[t.i]
		switch c {
		case '\\':
			t.escape(false)
		case '[':
			t.class(false)
		case '.':
			switch {
			case t.flags&reDotall_ != 0:
				t.out.WriteString(`(?s:.)`)
			case t.flags&reUnixLines_ != 0:
				t.out.WriteString(`[^\n]`)
			default:
				t.out.WriteString(`[^\n\r\x{85}\x{2028}\x{2029}]`)
			}
			t.i++
		case '$':
			t.eolGroup()
			t.i++
		case '(':
			t.group()
		default:
			_, n := utf8.DecodeRuneInString(p[t.i:])
			t.out.WriteString(p[t.i : t.i+n])
			t.i += n
		}
	}
	if t.errDesc != "" {
		return newPatternSyntaxException_(t.errDesc, p, t.errIdx)
	}
	pat.goSrc = t.out.String()
	re, err := regexp.Compile(pat.goSrc)
	if err != nil {
		desc := strings.TrimPrefix(err.Error(), "error parsing regexp: ")
		return newPatternSyntaxException_(desc, p, -1)
	}
	pat.re = re
	return nil
}

type patternKey_ struct {
	src   string
	flags int32
}

var patternCache_ = map[patternKey_]*Pattern{}

func Pattern_compile(regex string) *Pattern { return Pattern_compile_String_int(regex, 0) }

func Pattern_compile_String_int(regex string, flags int32) *Pattern {
	if regex == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	key := patternKey_{regex, flags}
	if p, ok := patternCache_[key]; ok {
		return p
	}
	pat := &Pattern{src: regex, flags_: flags}
	if ex := translateRegex_(pat); ex != nil {
		panic(ex.self())
	}
	if len(patternCache_) >= 1024 {
		patternCache_ = map[patternKey_]*Pattern{}
	}
	patternCache_[key] = pat
	return pat
}

func Pattern_quote(s string) string {
	if !strings.Contains(s, `\E`) {
		return `\Q` + s + `\E`
	}
	return `\Q` + strings.ReplaceAll(s, `\E`, `\E\\E\Q`) + `\E`
}

func (this *Pattern) pattern() string  { return this.src }
func (this *Pattern) toString() string { return this.src }
func (this *Pattern) flags() int32     { return this.flags_ }

func (this *Pattern) matcher(input any) *Matcher_regex {
	m := &Matcher_regex{p: this}
	m.reset(input)
	return m
}

func (this *Pattern) ctx_() *regexp.Regexp {
	if this.reCtx == nil {
		this.reCtx = regexp.MustCompile(`(?s:.)(?:` + this.goSrc + `)`)
	}
	return this.reCtx
}

func (this *Pattern) at_() *regexp.Regexp {
	if this.reAt == nil {
		this.reAt = regexp.MustCompile(`\A(?:` + this.goSrc + `)`)
	}
	return this.reAt
}

func (this *Pattern) full_() *regexp.Regexp {
	if this.reFull == nil {
		this.reFull = regexp.MustCompile(`\A(?:` + this.goSrc + `)\z`)
	}
	return this.reFull
}

// ---- Matcher

type Matcher_regex struct {
	p       *Pattern
	s       string
	ascii   bool
	rs, re  int   // region, byte offsets
	cur     []int // current match: absolute byte offsets of the Go groups
	done    bool  // the last find failed
	hitEnd_ bool
}

func (this *Matcher_regex) reset_() {
	this.cur = nil
	this.done = false
	this.hitEnd_ = false
}

func (this *Matcher_regex) reset(input any) *Matcher_regex {
	this.s = csString_(input)
	this.ascii = allASCII_(this.s)
	this.rs, this.re = 0, len(this.s)
	this.reset_()
	return this
}

func (this *Matcher_regex) units_(off int) int32 {
	if this.ascii {
		return int32(off)
	}
	return unitsBefore_(this.s, off)
}

func (this *Matcher_regex) region(start int32, end int32) *Matcher_regex {
	n := String_length(this.s)
	if start < 0 || start > n {
		panic(new_IndexOutOfBoundsException_String("start").self())
	}
	if end < 0 || end > n {
		panic(new_IndexOutOfBoundsException_String("end").self())
	}
	if start > end {
		panic(new_IndexOutOfBoundsException_String("start > end").self())
	}
	this.reset_()
	if this.ascii {
		this.rs, this.re = int(start), int(end)
	} else {
		this.rs, _ = locate_(this.s, start)
		this.re, _ = locate_(this.s, end)
	}
	return this
}

func (this *Matcher_regex) regionStart() int32 { return this.units_(this.rs) }
func (this *Matcher_regex) regionEnd() int32   { return this.units_(this.re) }

// accept_ records a match found in the region text at byte offset base.
func (this *Matcher_regex) accept_(loc []int, base int) bool {
	if loc == nil {
		this.cur = nil
		this.hitEnd_ = true
		return false
	}
	for i := range loc {
		if loc[i] >= 0 {
			loc[i] += base
		}
	}
	this.hitEnd_ = loc[1] == this.re
	// A `$` that matched before a final line terminator consumed it: give it back.
	for _, g := range this.p.eol {
		a, b := loc[2*g], loc[2*g+1]
		if a >= 0 && b == loc[1] && a < b {
			for i := range loc {
				if loc[i] > a {
					loc[i] = a
				}
			}
		}
	}
	this.cur = loc
	return true
}

// splitCRLF_ reports that a `$` of the current match stands between the \r
// and the \n of a final \r\n, where Java's `$` does not match.
func (this *Matcher_regex) splitCRLF_() bool {
	if this.p.flags_&reUnixLines_ != 0 {
		return false
	}
	for _, g := range this.p.eol {
		a := this.cur[2*g]
		if a > this.rs && a == this.re-1 && this.s[a] == '\n' && this.s[a-1] == '\r' {
			return true
		}
	}
	return false
}

func (this *Matcher_regex) find() bool {
	if this.done {
		return false
	}
	pos := this.rs
	if this.cur != nil {
		pos = this.cur[1]
		if this.cur[0] == pos {
			pos++
		}
	}
	for {
		if pos > this.re {
			this.done = true
			this.cur = nil
			this.hitEnd_ = true
			return false
		}
		sub := this.s[this.rs:this.re]
		rel := pos - this.rs
		// Never start in the middle of a UTF-8 sequence.
		for rel < len(sub) && sub[rel]&0xC0 == 0x80 {
			rel++
		}
		var ok bool
		if rel == 0 {
			ok = this.accept_(this.p.re.FindStringSubmatchIndex(sub), this.rs)
		} else {
			_, w := utf8.DecodeLastRuneInString(sub[:rel])
			loc := this.p.ctx_().FindStringSubmatchIndex(sub[rel-w:])
			if loc != nil {
				_, n := utf8.DecodeRuneInString(sub[rel-w+loc[0]:])
				loc[0] += n
			}
			ok = this.accept_(loc, this.rs+rel-w)
		}
		if !ok {
			this.done = true
			return false
		}
		if !this.splitCRLF_() {
			return true
		}
		pos = this.cur[0] + 1
	}
}

func (this *Matcher_regex) lookingAt() bool {
	this.done = false
	if !this.accept_(this.p.at_().FindStringSubmatchIndex(this.s[this.rs:this.re]), this.rs) {
		return false
	}
	if this.splitCRLF_() {
		this.cur = nil
		return false
	}
	return true
}

func (this *Matcher_regex) matches() bool {
	this.done = false
	if !this.accept_(this.p.full_().FindStringSubmatchIndex(this.s[this.rs:this.re]), this.rs) {
		return false
	}
	if this.cur[1] != this.re || this.splitCRLF_() {
		this.cur = nil
		return false
	}
	return true
}

func (this *Matcher_regex) hitEnd() bool { return this.hitEnd_ }

func (this *Matcher_regex) need_() {
	if this.cur == nil {
		stateFail_("No match found")
	}
}

func (this *Matcher_regex) groupCount() int32 { return int32(len(this.p.groupMap) - 1) }

func (this *Matcher_regex) span_(group int32) (int, int) {
	this.need_()
	if group < 0 || int(group) >= len(this.p.groupMap) {
		panic(new_IndexOutOfBoundsException_String("No group " + istr_(int64(group))).self())
	}
	g := this.p.groupMap[group]
	return this.cur[2*g], this.cur[2*g+1]
}

func (this *Matcher_regex) start() int32 {
	this.need_()
	return this.units_(this.cur[0])
}

func (this *Matcher_regex) end() int32 {
	this.need_()
	return this.units_(this.cur[1])
}

func (this *Matcher_regex) start_int(group int32) int32 {
	a, _ := this.span_(group)
	if a < 0 {
		return -1
	}
	return this.units_(a)
}

func (this *Matcher_regex) end_int(group int32) int32 {
	_, b := this.span_(group)
	if b < 0 {
		return -1
	}
	return this.units_(b)
}

func (this *Matcher_regex) group(group int32) string {
	a, b := this.span_(group)
	if a < 0 {
		return nullstr_
	}
	return this.s[a:b]
}

func (this *Matcher_regex) group_0() string { return this.group(0) }

func (this *Matcher_regex) group_String(name string) string {
	this.need_()
	g, ok := this.p.names[name]
	if !ok {
		argFail_("No group with name <" + name + ">")
	}
	return this.group(int32(g))
}

func MatchResult_group_String(m MatchResult, name string) string {
	return m.(interface{ group_String(string) string }).group_String(name)
}

// appendReplacement_ expands a Java replacement string for the current match.
func (this *Matcher_regex) appendReplacement_(sb *strings.Builder, repl string) {
	for i := 0; i < len(repl); i++ {
		c := repl[i]
		switch c {
		case '\\':
			i++
			if i == len(repl) {
				argFail_("character to be escaped is missing")
			}
			sb.WriteByte(repl[i])
		case '$':
			i++
			if i == len(repl) {
				argFail_("Illegal group reference: group index is missing")
			}
			var g int32
			if repl[i] == '{' {
				end := strings.IndexByte(repl[i:], '}')
				if end < 0 {
					argFail_("named capturing group is missing trailing '}'")
				}
				name := repl[i+1 : i+end]
				n, ok := this.p.names[name]
				if !ok {
					argFail_("No group with name {" + name + "}")
				}
				g = int32(n)
				i += end
			} else {
				if repl[i] < '0' || repl[i] > '9' {
					argFail_("Illegal group reference")
				}
				g = int32(repl[i] - '0')
				if g > this.groupCount() {
					panic(new_IndexOutOfBoundsException_String("No group " + istr_(int64(g))).self())
				}
				for i+1 < len(repl) && repl[i+1] >= '0' && repl[i+1] <= '9' {
					next := g*10 + int32(repl[i+1]-'0')
					if next > this.groupCount() {
						break
					}
					g = next
					i++
				}
			}
			if a, b := this.span_(g); a >= 0 {
				sb.WriteString(this.s[a:b])
			}
		default:
			sb.WriteByte(c)
		}
	}
}

func (this *Matcher_regex) replace_(repl string, all bool) string {
	if repl == nullstr_ {
		panic(new_NullPointerException_String("replacement").self())
	}
	this.reset_()
	if !this.find() {
		return this.s
	}
	var sb strings.Builder
	last := 0
	for {
		sb.WriteString(this.s[last:this.cur[0]])
		this.appendReplacement_(&sb, repl)
		last = this.cur[1]
		if !all || !this.find() {
			break
		}
	}
	sb.WriteString(this.s[last:])
	return sb.String()
}

func (this *Matcher_regex) replaceAll(repl string) string { return this.replace_(repl, true) }

func (this *Matcher_regex) replaceFirst(repl string) string { return this.replace_(repl, false) }

func (this *Matcher_regex) toString() string {
	last := ""
	if this.cur != nil {
		last = this.s[this.cur[0]:this.cur[1]]
	}
	return "java.util.regex.Matcher[pattern=" + this.p.src + " region=" + istr_(int64(this.regionStart())) + "," + istr_(int64(this.regionEnd())) + " lastmatch=" + last + "]"
}

func Matcher_regex_quoteReplacement(s string) string {
	if !strings.ContainsAny(s, `\$`) {
		return s
	}
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' || s[i] == '$' {
			sb.WriteByte('\\')
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

// ---- the regex methods of String

func String_matches(s string, regex string) bool {
	return Pattern_compile(regex).matcher(s).matches()
}

func String_replaceAll(s string, regex string, repl string) string {
	return Pattern_compile(regex).matcher(s).replaceAll(repl)
}

func String_replaceFirst(s string, regex string, repl string) string {
	return Pattern_compile(regex).matcher(s).replaceFirst(repl)
}

func String_split(s string, regex string) []string { return String_split_String_int(s, regex, 0) }

func String_split_String_int(s string, regex string, limit int32) []string {
	var out []string
	limited := limit > 0
	lit := ""
	switch {
	case len(regex) == 1 && !strings.Contains(`.$|()[{^?*+\`, regex):
		lit = regex
	case len(regex) == 2 && regex[0] == '\\' && regex[1] < 0x80 &&
		!(regex[1] >= '0' && regex[1] <= '9' || regex[1] >= 'a' && regex[1] <= 'z' || regex[1] >= 'A' && regex[1] <= 'Z'):
		lit = regex[1:]
	default:
		if _, n := utf8.DecodeRuneInString(regex); n == len(regex) && n > 1 {
			lit = regex
		}
	}
	index := 0
	if lit != "" {
		for {
			p := strings.Index(s[index:], lit)
			if p < 0 {
				break
			}
			if !limited || len(out) < int(limit)-1 {
				out = append(out, s[index:index+p])
				index += p + len(lit)
			} else {
				break
			}
		}
		if index == 0 && len(out) == 0 {
			return []string{s}
		}
	} else {
		m := Pattern_compile(regex).matcher(s)
		for m.find() {
			if !limited || len(out) < int(limit)-1 {
				if index == 0 && m.cur[0] == 0 && m.cur[1] == 0 {
					continue
				}
				out = append(out, s[index:m.cur[0]])
				index = m.cur[1]
			} else {
				break
			}
		}
		if index == 0 && len(out) == 0 {
			return []string{s}
		}
	}
	out = append(out, s[index:])
	if limit == 0 {
		n := len(out)
		for n > 0 && out[n-1] == "" {
			n--
		}
		out = out[:n]
	}
	return out
}
