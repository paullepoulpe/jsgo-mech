package closure

// java.net.URI (RFC 2396 as the JDK reads it), URISyntaxException and URL.

import (
	"strings"
)

// ---- URISyntaxException

type URISyntaxException struct {
	Exception
	input_  string
	reason_ string
	index_  int32
}

func (this *URISyntaxException) as_URISyntaxException() *URISyntaxException { return this }

func to_URISyntaxException(v any) *URISyntaxException {
	if v == nil {
		return nil
	}
	return v.(interface {
		as_URISyntaxException() *URISyntaxException
	}).as_URISyntaxException()
}

func is_URISyntaxException(v any) bool {
	_, ok := v.(interface {
		as_URISyntaxException() *URISyntaxException
	})
	return ok
}

func (this *URISyntaxException) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *URISyntaxException) up_Exception() *Exception {
	if this == nil {
		return nil
	}
	return &this.Exception
}

func new_URISyntaxException_String_String_int(input string, reason string, index int32) *URISyntaxException {
	if input == nullstr_ || reason == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	if index < -1 {
		panic(new_IllegalArgumentException_0().self())
	}
	this := &URISyntaxException{input_: input, reason_: reason, index_: index}
	this.self_ = this
	this.init_Exception_String(reason)
	return this
}

func (this *URISyntaxException) getInput() string  { return this.input_ }
func (this *URISyntaxException) getReason() string { return this.reason_ }
func (this *URISyntaxException) getIndex() int32   { return this.index_ }

func (this *URISyntaxException) getMessage() string {
	m := this.reason_
	if this.index_ > -1 {
		m += " at index " + istr_(int64(this.index_))
	}
	return m + ": " + this.input_
}

// ---- URI

// URI keeps the components as they appear in the string (still escaped);
// a component that is absent is nullstr_.
type URI struct {
	string_    string
	scheme_    string
	ssp_       string // scheme-specific part
	authority_ string
	userInfo_  string
	host_      string
	port_      int32
	path_      string // nullstr_ for an opaque URI
	query_     string
	fragment_  string
}

func uriFail_(input string, reason string, index int) {
	// The index counts chars; the input before it is ASCII or counted in units.
	panic(new_URISyntaxException_String_String_int(input, reason, unitsBefore_(input, index)).self())
}

func isAlpha_(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isDigit_(c byte) bool { return c >= '0' && c <= '9' }
func isHex_(c byte) bool {
	return isDigit_(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

const uriUnreserved_ = "_-!.~'()*"
const uriPunct_ = ",;:$&+="

// checkURIChars_ checks that s[from:to] holds only characters legal in a
// component: letters, digits, the given punctuation, escapes, and "other"
// (non-ASCII, non-control, non-space) characters.
func checkURIChars_(input string, from int, to int, extra string, what string) {
	for i := from; i < to; {
		c := input[i]
		switch {
		case isAlpha_(c) || isDigit_(c) || strings.IndexByte(extra, c) >= 0:
			i++
		case c == '%':
			if i+2 >= to || !isHex_(input[i+1]) || !isHex_(input[i+2]) {
				uriFail_(input, "Malformed escape pair", i)
			}
			i += 3
		case c >= 0x80:
			cp, size := decode_(input, i)
			// ISO controls and space separators are not allowed.
			if cp < 0xA0 || cp == 0xA0 || cp == 0x1680 || cp >= 0x2000 && cp <= 0x200A || cp == 0x2028 || cp == 0x2029 || cp == 0x202F || cp == 0x205F || cp == 0x3000 {
				uriFail_(input, "Illegal character in "+what, i)
			}
			i += size
		default:
			uriFail_(input, "Illegal character in "+what, i)
		}
	}
}

func new_URI(str string) *URI {
	if str == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	u := &URI{string_: str, scheme_: nullstr_, ssp_: nullstr_, authority_: nullstr_, userInfo_: nullstr_, host_: nullstr_,
		port_: -1, path_: nullstr_, query_: nullstr_, fragment_: nullstr_}
	n := len(str)
	// [scheme:]
	p := 0
	end := strings.IndexAny(str, "/?#:")
	if end >= 0 && str[end] == ':' {
		if end == 0 {
			uriFail_(str, "Expected scheme name", 0)
		}
		if !isAlpha_(str[0]) {
			uriFail_(str, "Illegal character in scheme name", 0)
		}
		for i := 1; i < end; i++ {
			c := str[i]
			if !isAlpha_(c) && !isDigit_(c) && c != '+' && c != '-' && c != '.' {
				uriFail_(str, "Illegal character in scheme name", i)
			}
		}
		u.scheme_ = str[:end]
		p = end + 1
		if p >= n {
			uriFail_(str, "Expected scheme-specific part", p)
		}
	}
	sspStart := p
	fragAt := strings.IndexByte(str[p:], '#')
	sspEnd := n
	if fragAt >= 0 {
		sspEnd = p + fragAt
		checkURIChars_(str, sspEnd+1, n, uriUnreserved_+uriPunct_+"?/[]@", "fragment")
		u.fragment_ = str[sspEnd+1:]
	}
	u.ssp_ = str[sspStart:sspEnd]
	if u.scheme_ != nullstr_ && (p >= sspEnd || str[p] != '/') {
		// Opaque.
		if p >= sspEnd {
			uriFail_(str, "Expected scheme-specific part", p)
		}
		checkURIChars_(str, p, sspEnd, uriUnreserved_+uriPunct_+"?/[]@", "opaque part")
		return u
	}
	// Hierarchical: [//authority][path][?query]
	if strings.HasPrefix(str[p:sspEnd], "//") {
		p += 2
		q := p
		for q < sspEnd && str[q] != '/' && str[q] != '?' {
			q++
		}
		if q > p {
			u.parseAuthority_(str, p, q)
		} else if q >= sspEnd {
			uriFail_(str, "Expected authority", p)
		}
		p = q
	}
	q := p
	for q < sspEnd && str[q] != '?' {
		q++
	}
	checkURIChars_(str, p, q, uriUnreserved_+uriPunct_+"/@", "path")
	u.path_ = str[p:q]
	if q < sspEnd {
		checkURIChars_(str, q+1, sspEnd, uriUnreserved_+uriPunct_+"?/[]@", "query")
		u.query_ = str[q+1 : sspEnd]
	}
	return u
}

// parseAuthority_ reads [userinfo@]host[:port]; an authority that is not of
// that form is kept as a registry-based authority (no host).
func (u *URI) parseAuthority_(str string, p int, q int) {
	u.authority_ = str[p:q]
	checkURIChars_(str, p, q, uriUnreserved_+uriPunct_+"@[]", "authority")
	a := u.authority_
	rest := a
	user := nullstr_
	if i := strings.LastIndexByte(rest, '@'); i >= 0 {
		user, rest = rest[:i], rest[i+1:]
	}
	host, port := rest, ""
	if strings.HasPrefix(rest, "[") {
		i := strings.IndexByte(rest, ']')
		if i < 0 {
			uriFail_(str, "Expected closing bracket for IPv6 address", q)
		}
		host, port = rest[:i+1], rest[i+1:]
		if port != "" && port[0] != ':' {
			return
		}
		port = strings.TrimPrefix(port, ":")
	} else if i := strings.LastIndexByte(rest, ':'); i >= 0 {
		host, port = rest[:i], rest[i+1:]
	}
	if host == "" {
		return
	}
	pn := int32(-1)
	if port != "" {
		pn = 0
		for i := 0; i < len(port); i++ {
			if !isDigit_(port[i]) || pn > 99999999 {
				return
			}
			pn = pn*10 + int32(port[i]-'0')
		}
	}
	if !strings.HasPrefix(host, "[") {
		for i := 0; i < len(host); i++ {
			c := host[i]
			if !isAlpha_(c) && !isDigit_(c) && c != '-' && c != '.' {
				return
			}
		}
	}
	u.userInfo_, u.host_, u.port_ = user, host, pn
}

// uriDecode_ replaces escaped octets by the characters they encode in UTF-8.
func uriDecode_(s string) string {
	if s == nullstr_ || strings.IndexByte(s, '%') < 0 {
		return s
	}
	hexv := func(c byte) byte {
		switch {
		case c >= 'a':
			return c - 'a' + 10
		case c >= 'A':
			return c - 'A' + 10
		}
		return c - '0'
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		if s[i] != '%' {
			out = append(out, s[i])
			i++
			continue
		}
		// A run of escapes is decoded as UTF-8 as a whole.
		var run []byte
		for i+2 < len(s) && s[i] == '%' {
			run = append(run, hexv(s[i+1])<<4|hexv(s[i+2]))
			i += 3
		}
		d, _ := decodeUTF8_(run)
		out = append(out, d...)
		if len(run) == 0 {
			out = append(out, s[i])
			i++
		}
	}
	return string(out)
}

func (this *URI) toString() string { return this.string_ }
func (this *URI) toASCIIString() string {
	return uriQuote_(this.string_, uriUnreserved_+uriPunct_+"?/[]@#%:")
}

func (this *URI) getScheme() string       { return this.scheme_ }
func (this *URI) isAbsolute() bool        { return this.scheme_ != nullstr_ }
func (this *URI) isOpaque() bool          { return this.path_ == nullstr_ }
func (this *URI) getHost() string         { return this.host_ }
func (this *URI) getPort() int32          { return this.port_ }
func (this *URI) getRawPath() string      { return this.path_ }
func (this *URI) getPath() string         { return uriDecode_(this.path_) }
func (this *URI) getRawQuery() string     { return this.query_ }
func (this *URI) getQuery() string        { return uriDecode_(this.query_) }
func (this *URI) getRawFragment() string  { return this.fragment_ }
func (this *URI) getFragment() string     { return uriDecode_(this.fragment_) }
func (this *URI) getRawAuthority() string { return this.authority_ }
func (this *URI) getAuthority() string    { return uriDecode_(this.authority_) }
func (this *URI) getRawUserInfo() string  { return this.userInfo_ }
func (this *URI) getUserInfo() string     { return uriDecode_(this.userInfo_) }

func (this *URI) getRawSchemeSpecificPart() string { return this.ssp_ }
func (this *URI) getSchemeSpecificPart() string    { return uriDecode_(this.ssp_) }

// normalize removes "." segments and "name/.." pairs from the path.
func (this *URI) normalize() *URI {
	if this.isOpaque() || this.path_ == "" {
		return this
	}
	path := this.path_
	abs := strings.HasPrefix(path, "/")
	segs := strings.Split(strings.TrimPrefix(path, "/"), "/")
	trailing := false
	out := make([]string, 0, len(segs))
	for i, s := range segs {
		last := i == len(segs)-1
		switch {
		case s == ".":
			trailing = last
		case s == "":
			if last {
				trailing = true
			}
		case s == "..":
			if len(out) > 0 && out[len(out)-1] != ".." {
				out = out[:len(out)-1]
				trailing = last
			} else if !abs {
				out = append(out, s)
			} else {
				out = append(out, s)
			}
		default:
			out = append(out, s)
		}
	}
	np := strings.Join(out, "/")
	if abs {
		np = "/" + np
	}
	if trailing && np != "" && !strings.HasSuffix(np, "/") {
		np += "/"
	}
	// A first segment with a colon would read as a scheme.
	if !abs && this.scheme_ == nullstr_ && len(out) > 0 && strings.Contains(out[0], ":") {
		np = "./" + np
	}
	if np == path {
		return this
	}
	return this.withPath_(np)
}

func (this *URI) withPath_(path string) *URI {
	var b strings.Builder
	if this.scheme_ != nullstr_ {
		b.WriteString(this.scheme_ + ":")
	}
	if this.authority_ != nullstr_ {
		b.WriteString("//" + this.authority_)
	}
	b.WriteString(path)
	if this.query_ != nullstr_ {
		b.WriteString("?" + this.query_)
	}
	if this.fragment_ != nullstr_ {
		b.WriteString("#" + this.fragment_)
	}
	return new_URI(b.String())
}

// resolve_URI resolves the given URI against this one (RFC 2396, 5.2).
func (this *URI) resolve_URI(child *URI) *URI {
	if child.isOpaque() || this.isOpaque() {
		return child
	}
	// A reference to the current document.
	if child.scheme_ == nullstr_ && child.authority_ == nullstr_ && child.path_ == "" && child.fragment_ != nullstr_ && child.query_ == nullstr_ {
		if this.fragment_ != nullstr_ && this.fragment_ == child.fragment_ {
			return this
		}
		s := this.string_
		if i := strings.IndexByte(s, '#'); i >= 0 {
			s = s[:i]
		}
		return new_URI(s + "#" + child.fragment_)
	}
	if child.scheme_ != nullstr_ {
		return child
	}
	var b strings.Builder
	if this.scheme_ != nullstr_ {
		b.WriteString(this.scheme_ + ":")
	}
	if child.authority_ != nullstr_ {
		b.WriteString("//" + child.authority_ + child.path_)
	} else {
		if this.authority_ != nullstr_ {
			b.WriteString("//" + this.authority_)
		}
		if strings.HasPrefix(child.path_, "/") {
			b.WriteString(child.path_)
		} else {
			base := this.path_
			if i := strings.LastIndexByte(base, '/'); i >= 0 {
				base = base[:i+1]
			} else {
				base = ""
			}
			merged := (&URI{scheme_: nullstr_, authority_: nullstr_, query_: nullstr_, fragment_: nullstr_, path_: base + child.path_, string_: base + child.path_}).normalize()
			b.WriteString(merged.path_)
		}
	}
	if child.query_ != nullstr_ {
		b.WriteString("?" + child.query_)
	}
	if child.fragment_ != nullstr_ {
		b.WriteString("#" + child.fragment_)
	}
	return new_URI(b.String())
}

func (this *URI) resolve(str string) *URI { return this.resolve_URI(URI_create(str)) }

func (this *URI) toURL() *URL {
	if !this.isAbsolute() {
		panic(new_IllegalArgumentException_String("URI is not absolute").self())
	}
	return &URL{spec_: this.string_}
}

func cmpNullable_(a, b string, fold bool) int32 {
	switch {
	case a == b:
		return 0
	case a == nullstr_:
		return -1
	case b == nullstr_:
		return 1
	}
	if fold {
		a, b = strings.ToLower(a), strings.ToLower(b)
	}
	return String_compareTo(a, b)
}

// compareTo orders by scheme (ignoring case), hierarchical before opaque,
// then by the components.
func (this *URI) compareTo(o any) int32 {
	that := o.(*URI)
	if c := cmpNullable_(this.scheme_, that.scheme_, true); c != 0 {
		return c
	}
	if this.isOpaque() != that.isOpaque() {
		if this.isOpaque() {
			return 1
		}
		return -1
	}
	if this.isOpaque() {
		if c := cmpNullable_(this.ssp_, that.ssp_, false); c != 0 {
			return c
		}
		return cmpNullable_(this.fragment_, that.fragment_, false)
	}
	if this.host_ != nullstr_ && that.host_ != nullstr_ {
		if c := cmpNullable_(this.userInfo_, that.userInfo_, false); c != 0 {
			return c
		}
		if c := cmpNullable_(this.host_, that.host_, true); c != 0 {
			return c
		}
		if this.port_ != that.port_ {
			return this.port_ - that.port_
		}
	} else if c := cmpNullable_(this.authority_, that.authority_, false); c != 0 {
		return c
	}
	if c := cmpNullable_(this.path_, that.path_, false); c != 0 {
		return c
	}
	if c := cmpNullable_(this.query_, that.query_, false); c != 0 {
		return c
	}
	return cmpNullable_(this.fragment_, that.fragment_, false)
}

func (this *URI) equals(o any) bool {
	that, ok := o.(*URI)
	return ok && that != nil && this.compareTo(that) == 0
}

func (this *URI) hashCode() int32 {
	h := int32(0)
	add := func(s string, fold bool) {
		if s == nullstr_ {
			return
		}
		if fold {
			s = strings.ToLower(s)
		}
		h = h*127 + String_hashCode(s)
	}
	add(this.scheme_, true)
	add(this.fragment_, false)
	if this.isOpaque() {
		add(this.ssp_, false)
		return h
	}
	add(this.path_, false)
	add(this.query_, false)
	if this.host_ != nullstr_ {
		add(this.userInfo_, false)
		add(this.host_, true)
		h += 1949 * this.port_
	} else {
		add(this.authority_, false)
	}
	return h
}

// ---- URL

// URL is a location: of an embedded resource ("jar:file:/closure-compiler.jar!/name")
// or of a file ("file:/path").
type URL struct {
	spec_     string
	resource_ string // the absolute name of an embedded resource, or ""
}

const resourceURLPrefix_ = "jar:file:/closure-compiler.jar!/"

func resourceURL_(name string) *URL {
	return &URL{spec_: resourceURLPrefix_ + name, resource_: name}
}

func new_URL(spec string) *URL {
	if spec == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	if strings.HasPrefix(spec, resourceURLPrefix_) {
		return resourceURL_(spec[len(resourceURLPrefix_):])
	}
	return &URL{spec_: spec}
}

func (this *URL) toString() string       { return this.spec_ }
func (this *URL) toExternalForm() string { return this.spec_ }
func (this *URL) hashCode() int32        { return String_hashCode(this.spec_) }
func (this *URL) toURI() *URI            { return new_URI(this.spec_) }

func (this *URL) equals(o any) bool {
	u, ok := o.(*URL)
	return ok && u != nil && u.spec_ == this.spec_
}

func (this *URL) getProtocol() string {
	if i := strings.IndexByte(this.spec_, ':'); i >= 0 {
		return this.spec_[:i]
	}
	return ""
}

func (this *URL) getPath() string {
	s := this.spec_[strings.IndexByte(this.spec_, ':')+1:]
	if i := strings.IndexAny(s, "?#"); i >= 0 && this.resource_ == "" {
		s = s[:i]
	}
	return s
}

func (this *URL) getFile() string { return this.getPath() }

func (this *URL) openStream() *InputStream {
	if this.resource_ != "" {
		b, ok := resourceBytes_(this.resource_)
		if !ok {
			panic(new_FileNotFoundException_String(this.spec_).self())
		}
		return new_ByteArrayInputStream(i8_(b)).up_InputStream()
	}
	if strings.HasPrefix(this.spec_, "file:") {
		p := strings.TrimPrefix(this.spec_, "file:")
		if strings.HasPrefix(p, "//") {
			p = p[2:]
			if i := strings.IndexByte(p, '/'); i >= 0 {
				p = p[i:]
			}
		}
		return new_FileInputStream(uriDecode_(p)).up_InputStream()
	}
	panic(new_IOException_String("unsupported URL: " + this.spec_).self())
}
