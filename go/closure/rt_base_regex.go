package closure

// com.google.re2j on Go's regexp package. Both implement RE2, so a pattern is
// passed through unchanged. Matching is done on the UTF-8 bytes; every index a
// Matcher takes or returns is converted from/to UTF-16 units.

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// re2j's flag bits.
const (
	reCaseInsensitive_ = 1
	reDotall_          = 2
	reMultiline_       = 4
)

type Pattern struct {
	src    string
	flags_ int32
	goSrc  string
	re     *regexp.Regexp
	reCtx  *regexp.Regexp // one character of left context, then the pattern
	reAt   *regexp.Regexp // anchored at the start
	reFull *regexp.Regexp // anchored at both ends
	names  map[string]int
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
	pat := &Pattern{src: regex, flags_: flags, names: map[string]int{}}
	prefix := ""
	if flags&reCaseInsensitive_ != 0 {
		prefix += "i"
	}
	if flags&reDotall_ != 0 {
		prefix += "s"
	}
	if flags&reMultiline_ != 0 {
		prefix += "m"
	}
	pat.goSrc = regex
	if prefix != "" {
		pat.goSrc = "(?" + prefix + ")" + regex
	}
	re, err := regexp.Compile(pat.goSrc)
	if err != nil {
		ex := &PatternSyntaxException{}
		ex.self_ = ex
		ex.init_PatternSyntaxException_String(strings.TrimPrefix(err.Error(), "error parsing regexp: "))
		panic(ex.self())
	}
	pat.re = re
	for i, n := range re.SubexpNames() {
		if n != "" {
			pat.names[n] = i
		}
	}
	if len(patternCache_) >= 1024 {
		patternCache_ = map[patternKey_]*Pattern{}
	}
	patternCache_[key] = pat
	return pat
}

func Pattern_quote(s string) string { return regexp.QuoteMeta(s) }

func Pattern_matches(regex string, input any) bool {
	return Pattern_compile(regex).matcher(input).matches()
}

func (this *Pattern) pattern() string  { return this.src }
func (this *Pattern) toString() string { return this.src }
func (this *Pattern) flags() int32     { return this.flags_ }

func (this *Pattern) matcher(input any) *Matcher_re2j {
	m := &Matcher_re2j{p: this}
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

func (this *Pattern) split(input string) []string { return this.split_String_int(input, 0) }

// split_String_int follows String.split: a leading empty match adds nothing,
// and with limit 0 trailing empty strings are dropped.
func (this *Pattern) split_String_int(s string, limit int32) []string {
	var out []string
	limited := limit > 0
	index := 0
	m := this.matcher(s)
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

// ---- Matcher

type Matcher_re2j struct {
	p      *Pattern
	s      string
	ascii  bool
	rs, re int   // region, byte offsets
	cur    []int // current match: absolute byte offsets of the Go groups
	done   bool  // the last find failed
}

func (this *Matcher_re2j) reset_() {
	this.cur = nil
	this.done = false
}

func (this *Matcher_re2j) reset(input any) *Matcher_re2j {
	this.s = csString_(input)
	this.ascii = allASCII_(this.s)
	this.rs, this.re = 0, len(this.s)
	this.reset_()
	return this
}

func (this *Matcher_re2j) units_(off int) int32 {
	if this.ascii {
		return int32(off)
	}
	return unitsBefore_(this.s, off)
}

// accept_ records a match found in the region text at byte offset base.
func (this *Matcher_re2j) accept_(loc []int, base int) bool {
	if loc == nil {
		this.cur = nil
		return false
	}
	for i := range loc {
		if loc[i] >= 0 {
			loc[i] += base
		}
	}
	this.cur = loc
	return true
}

func (this *Matcher_re2j) find() bool {
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
		return true
	}
}

func (this *Matcher_re2j) lookingAt() bool {
	this.done = false
	if !this.accept_(this.p.at_().FindStringSubmatchIndex(this.s[this.rs:this.re]), this.rs) {
		return false
	}
	return true
}

func (this *Matcher_re2j) matches() bool {
	this.done = false
	if !this.accept_(this.p.full_().FindStringSubmatchIndex(this.s[this.rs:this.re]), this.rs) {
		return false
	}
	if this.cur[1] != this.re {
		this.cur = nil
		return false
	}
	return true
}

func (this *Matcher_re2j) need_() {
	if this.cur == nil {
		stateFail_("No match found")
	}
}

func (this *Matcher_re2j) groupCount() int32 { return int32(this.p.re.NumSubexp()) }

func (this *Matcher_re2j) span_(group int32) (int, int) {
	this.need_()
	if group < 0 || group > this.groupCount() {
		panic(new_IndexOutOfBoundsException_String("No group " + istr_(int64(group))).self())
	}
	return this.cur[2*group], this.cur[2*group+1]
}

func (this *Matcher_re2j) start() int32 {
	this.need_()
	return this.units_(this.cur[0])
}

func (this *Matcher_re2j) end() int32 {
	this.need_()
	return this.units_(this.cur[1])
}

func (this *Matcher_re2j) start_int(group int32) int32 {
	a, _ := this.span_(group)
	if a < 0 {
		return -1
	}
	return this.units_(a)
}

func (this *Matcher_re2j) end_int(group int32) int32 {
	_, b := this.span_(group)
	if b < 0 {
		return -1
	}
	return this.units_(b)
}

func (this *Matcher_re2j) group(group int32) string {
	a, b := this.span_(group)
	if a < 0 {
		return nullstr_
	}
	return this.s[a:b]
}

func (this *Matcher_re2j) group_0() string { return this.group(0) }

func (this *Matcher_re2j) group_String(name string) string {
	this.need_()
	g, ok := this.p.names[name]
	if !ok {
		argFail_("No group with name <" + name + ">")
	}
	return this.group(int32(g))
}

// appendReplacement_ expands a replacement string ($1, ${name}, \\$) for the current match.
func (this *Matcher_re2j) appendReplacement_(sb *strings.Builder, repl string) {
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

func (this *Matcher_re2j) replace_(repl string, all bool) string {
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

func (this *Matcher_re2j) replaceAll(repl string) string { return this.replace_(repl, true) }

func (this *Matcher_re2j) replaceFirst(repl string) string { return this.replace_(repl, false) }

func (this *Matcher_re2j) toString() string {
	last := ""
	if this.cur != nil {
		last = this.s[this.cur[0]:this.cur[1]]
	}
	return "com.google.re2j.Matcher[pattern=" + this.p.src + " lastmatch=" + last + "]"
}

