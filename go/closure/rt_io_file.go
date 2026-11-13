package closure

// java.io.File and java.nio.file (Path, Files, FileSystem, glob matchers, the
// file tree walk), with the semantics of the default file system on Unix.

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"
)

func init() {
	javaNames_["unixPath_"] = "sun.nio.fs.UnixPath"
	javaNames_["FileSystemException"] = "java.nio.file.FileSystemException"
	javaNames_["NoSuchFileException"] = "java.nio.file.NoSuchFileException"
	javaNames_["AccessDeniedException"] = "java.nio.file.AccessDeniedException"
	javaNames_["FileAlreadyExistsException"] = "java.nio.file.FileAlreadyExistsException"
	javaNames_["MalformedInputException"] = "java.nio.charset.MalformedInputException"
	javaNames_["UnmappableCharacterException"] = "java.nio.charset.UnmappableCharacterException"
	javaNames_["UnsupportedCharsetException"] = "java.nio.charset.UnsupportedCharsetException"
	javaNames_["ByteArrayInputStream"] = "java.io.ByteArrayInputStream"
	registerEnum_("java.nio.file.FileVisitResult", []any{FileVisitResult_CONTINUE, FileVisitResult_TERMINATE, FileVisitResult_SKIP_SUBTREE, FileVisitResult_SKIP_SIBLINGS})
	registerEnum_("java.nio.file.FileVisitOption", []any{FileVisitOption_FOLLOW_LINKS})
	registerEnum_("java.nio.file.LinkOption", []any{LinkOption_NOFOLLOW_LINKS})
	registerEnum_("java.nio.file.StandardOpenOption", []any{StandardOpenOption_READ, StandardOpenOption_WRITE, StandardOpenOption_APPEND,
		StandardOpenOption_TRUNCATE_EXISTING, StandardOpenOption_CREATE, StandardOpenOption_CREATE_NEW, StandardOpenOption_DELETE_ON_CLOSE,
		StandardOpenOption_SPARSE, StandardOpenOption_SYNC, StandardOpenOption_DSYNC})
	registerEnum_("com.google.common.io.FileWriteMode", []any{FileWriteMode_APPEND})
}

// ---- exceptions of java.nio.file

type FileSystemException struct {
	IOException
	file_ string
}

func (this *FileSystemException) getFile() string { return this.file_ }

type NoSuchFileException struct{ FileSystemException }
type AccessDeniedException struct{ FileSystemException }
type FileAlreadyExistsException struct{ FileSystemException }

// nioError_ is the exception java.nio.file throws for a failed operation on
// the named file.
func nioError_(file string, err error) any {
	switch {
	case errors.Is(err, syscall.ENOENT):
		e := &NoSuchFileException{}
		e.self_, e.file_ = e, file
		e.init_IOException_String(file)
		return e
	case errors.Is(err, syscall.EACCES), errors.Is(err, syscall.EPERM):
		e := &AccessDeniedException{}
		e.self_, e.file_ = e, file
		e.init_IOException_String(file)
		return e
	case errors.Is(err, syscall.EEXIST):
		e := &FileAlreadyExistsException{}
		e.self_, e.file_ = e, file
		e.init_IOException_String(file)
		return e
	}
	e := &FileSystemException{}
	e.self_, e.file_ = e, file
	e.init_IOException_String(file + ": " + osMessage_(err))
	return e
}

// ---- path strings

// cleanSlashes_ removes repeated and trailing separators, as both File and
// Path do when they are created.
func cleanSlashes_(s string) string {
	if !strings.Contains(s, "//") && (len(s) <= 1 || s[len(s)-1] != '/') {
		return s
	}
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '/' && len(b) > 0 && b[len(b)-1] == '/' {
			continue
		}
		b = append(b, s[i])
	}
	if len(b) > 1 && b[len(b)-1] == '/' {
		b = b[:len(b)-1]
	}
	return string(b)
}

func userDir_() string { return System_getProperty("user.dir", "/") }

// ---- java.io.File

var File_separator string = "/"
var File_separatorChar uint16 = '/'
var File_pathSeparator string = ":"
var File_pathSeparatorChar uint16 = ':'

type File struct {
	path_ string
}

func new_File(path string) *File {
	if path == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	return &File{path_: cleanSlashes_(path)}
}

func new_File_String_String(parent string, child string) *File {
	if child == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	if parent == nullstr_ {
		return new_File(child)
	}
	return &File{path_: cleanSlashes_(cleanSlashes_(parent) + "/" + child)}
}

func new_File_File_String(parent *File, child string) *File {
	if parent == nil {
		return new_File(child)
	}
	return new_File_String_String(parent.path_, child)
}

func (this *File) getPath() string  { return this.path_ }
func (this *File) toString() string { return this.path_ }

func (this *File) getName() string {
	return this.path_[strings.LastIndexByte(this.path_, '/')+1:]
}

func (this *File) getParent() string {
	i := strings.LastIndexByte(this.path_, '/')
	if i < 0 {
		return nullstr_
	}
	if i == 0 {
		if len(this.path_) > 1 {
			return "/"
		}
		return nullstr_
	}
	return this.path_[:i]
}

func (this *File) getParentFile() *File {
	p := this.getParent()
	if p == nullstr_ {
		return nil
	}
	return &File{path_: p}
}

func (this *File) isAbsolute() bool { return strings.HasPrefix(this.path_, "/") }

func (this *File) getAbsolutePath() string {
	if this.isAbsolute() {
		return this.path_
	}
	if this.path_ == "" {
		return userDir_()
	}
	return cleanSlashes_(userDir_() + "/" + this.path_)
}

func (this *File) getAbsoluteFile() *File { return &File{path_: this.getAbsolutePath()} }

func (this *File) getCanonicalPath() string {
	abs := pathOf_(this.getAbsolutePath()).normalize().(*unixPath_).s_
	// Resolve symbolic links of the longest prefix that exists.
	rest := ""
	for p := abs; ; {
		if r, err := realpath_(p); err == nil {
			return cleanSlashes_(r + rest)
		}
		i := strings.LastIndexByte(p, '/')
		if i <= 0 {
			return abs
		}
		rest = p[i:] + rest
		p = p[:i]
	}
}

func (this *File) getCanonicalFile() *File { return &File{path_: this.getCanonicalPath()} }

func (this *File) stat_() os.FileInfo {
	st, err := os.Stat(this.path_)
	if err != nil {
		return nil
	}
	return st
}

func (this *File) exists() bool { return this.stat_() != nil }

func (this *File) isDirectory() bool {
	st := this.stat_()
	return st != nil && st.IsDir()
}

func (this *File) isFile() bool {
	st := this.stat_()
	return st != nil && st.Mode().IsRegular()
}

func (this *File) canRead() bool { return syscall.Access(this.path_, 4) == nil }

func (this *File) canWrite() bool { return syscall.Access(this.path_, 2) == nil }

func (this *File) length() int64 {
	if st := this.stat_(); st != nil {
		return st.Size()
	}
	return 0
}

func (this *File) lastModified() int64 {
	if st := this.stat_(); st != nil {
		return st.ModTime().UnixMilli()
	}
	return 0
}

// list returns the names in the directory, or nil if this is not a directory.
func (this *File) list() []string {
	names, ok := dirNames_(this.path_)
	if !ok {
		return nil
	}
	return names
}

func (this *File) listFiles() []*File {
	names, ok := dirNames_(this.path_)
	if !ok {
		return nil
	}
	out := make([]*File, len(names))
	for i, n := range names {
		out[i] = new_File_File_String(this, n)
	}
	return out
}

func (this *File) mkdir() bool { return os.Mkdir(this.path_, 0o777) == nil }

func (this *File) mkdirs() bool {
	if this.exists() {
		return false
	}
	return os.MkdirAll(this.path_, 0o777) == nil
}

func (this *File) delete() bool { return os.Remove(this.path_) == nil }

func (this *File) renameTo(dest *File) bool { return os.Rename(this.path_, dest.path_) == nil }

// createNewFile creates the file if it does not exist yet, and says whether it did.
func (this *File) createNewFile() bool {
	f, err := os.OpenFile(this.path_, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		if errors.Is(err, syscall.EEXIST) {
			return false
		}
		panic(new_IOException_String(osMessage_(err)).self())
	}
	f.Close()
	return true
}

func (this *File) toPath() Path { return pathOf_(this.path_) }

func (this *File) toURI() *URI {
	p := this.getAbsolutePath()
	if this.isDirectory() && !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return newURIParts_("file", nullstr_, p, nullstr_, nullstr_)
}

func (this *File) equals(o any) bool {
	f, ok := o.(*File)
	return ok && f != nil && f.path_ == this.path_
}

func (this *File) hashCode() int32 { return String_hashCode(this.path_) ^ 1234321 }

func (this *File) compareTo(o any) int32 { return String_compareTo(this.path_, o.(*File).path_) }

// dirNames_ lists a directory in the order the operating system returns it,
// which is the order Java iterates it in.
func dirNames_(dir string) ([]string, bool) {
	if dir == "" {
		dir = "."
	}
	f, err := os.Open(dir)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	names, err := f.Readdirnames(-1)
	if err != nil {
		return nil, false
	}
	return names, true
}

func realpath_(p string) (string, error) {
	// A small realpath: resolve links component by component.
	if _, err := os.Lstat(p); err != nil {
		return "", err
	}
	parts := strings.Split(p, "/")
	cur := ""
	for depth, i := 0, 0; i < len(parts); i++ {
		name := parts[i]
		if name == "" || name == "." {
			continue
		}
		if name == ".." {
			if j := strings.LastIndexByte(cur, '/'); j >= 0 {
				cur = cur[:j]
			}
			continue
		}
		next := cur + "/" + name
		st, err := os.Lstat(next)
		if err != nil {
			return "", err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			depth++
			if depth > 40 {
				return "", syscall.ELOOP
			}
			target, err := os.Readlink(next)
			if err != nil {
				return "", err
			}
			rest := append(strings.Split(target, "/"), parts[i+1:]...)
			if strings.HasPrefix(target, "/") {
				cur = ""
			}
			parts, i = rest, -1
			continue
		}
		cur = next
	}
	if cur == "" {
		cur = "/"
	}
	return cur, nil
}

// ---- java.nio.file.Path

type unixPath_ struct {
	s_ string
}

func pathOf_(s string) *unixPath_ {
	if strings.IndexByte(s, 0) >= 0 {
		// java.nio.file.InvalidPathException is an IllegalArgumentException.
		panic(new_IllegalArgumentException_String("Nul character not allowed: " + s).self())
	}
	return &unixPath_{cleanSlashes_(s)}
}

func asPath_(p Path) *unixPath_ {
	if p == nil {
		panic(new_NullPointerException_0().self())
	}
	return p.(*unixPath_)
}

// optPath_ is a Path that is null when p is nil.
func optPath_(p *unixPath_) Path {
	if p == nil {
		return nil
	}
	return p
}

func Path_of(first string, more []string) Path {
	if first == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	s := first
	for _, m := range more {
		if m == nullstr_ {
			panic(new_NullPointerException_0().self())
		}
		if m == "" {
			continue
		}
		if s != "" {
			s += "/"
		}
		s += m
	}
	return pathOf_(s)
}

func Paths_get(first string, more []string) Path { return Path_of(first, more) }

func Path_resolve(p Path, other string) Path { return p.resolve_Path(pathOf_(other)) }

func Path_resolveSibling(p Path, other string) Path {
	return asPath_(p).resolveSibling_Path(pathOf_(other))
}

func Path_toFile(p Path) *File { return new_File(asPath_(p).s_) }

func Path_startsWith(p Path, other string) bool { return asPath_(p).startsWith(pathOf_(other)) }

func Path_endsWith(p Path, other string) bool { return asPath_(p).endsWith(pathOf_(other)) }

func (this *unixPath_) toString() string { return this.s_ }

func (this *unixPath_) isAbsolute() bool { return strings.HasPrefix(this.s_, "/") }

// names_ are the name elements: none for the root, one empty name for the
// empty path.
func (this *unixPath_) names_() []string {
	if this.s_ == "" {
		return []string{""}
	}
	s := strings.TrimPrefix(this.s_, "/")
	if s == "" {
		return nil
	}
	return strings.Split(s, "/")
}

func (this *unixPath_) getRoot() Path {
	if this.isAbsolute() {
		return &unixPath_{"/"}
	}
	return nil
}

func (this *unixPath_) getFileName() Path {
	if this.s_ == "/" {
		return nil
	}
	return &unixPath_{this.s_[strings.LastIndexByte(this.s_, '/')+1:]}
}

func (this *unixPath_) getParent() Path {
	i := strings.LastIndexByte(this.s_, '/')
	switch {
	case i < 0, this.s_ == "/":
		return nil
	case i == 0:
		return &unixPath_{"/"}
	}
	return &unixPath_{this.s_[:i]}
}

func (this *unixPath_) getNameCount() int32 { return int32(len(this.names_())) }

func (this *unixPath_) getName(i int32) Path {
	names := this.names_()
	if i < 0 || int(i) >= len(names) {
		panic(new_IllegalArgumentException_0().self())
	}
	return &unixPath_{names[i]}
}

func (this *unixPath_) subpath(a int32, b int32) Path {
	names := this.names_()
	if a < 0 || int(b) > len(names) || a >= b {
		panic(new_IllegalArgumentException_0().self())
	}
	return &unixPath_{strings.Join(names[a:b], "/")}
}

func (this *unixPath_) startsWith(other Path) bool {
	o, ok := other.(*unixPath_)
	if !ok || o == nil || this.isAbsolute() != o.isAbsolute() {
		return false
	}
	a, b := this.names_(), o.names_()
	if o.s_ == "" {
		return this.s_ == ""
	}
	if len(b) > len(a) {
		return false
	}
	for i := range b {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (this *unixPath_) endsWith(other Path) bool {
	o, ok := other.(*unixPath_)
	if !ok || o == nil {
		return false
	}
	if o.isAbsolute() {
		return this.s_ == o.s_
	}
	if o.s_ == "" {
		return this.s_ == ""
	}
	a, b := this.names_(), o.names_()
	if len(b) > len(a) {
		return false
	}
	for i := range b {
		if a[len(a)-len(b)+i] != b[i] {
			return false
		}
	}
	return true
}

func (this *unixPath_) normalize() Path {
	names := this.names_()
	if this.s_ == "" {
		return this
	}
	abs := this.isAbsolute()
	out := make([]string, 0, len(names))
	for _, n := range names {
		switch {
		case n == ".":
		case n == "..":
			if len(out) > 0 && out[len(out)-1] != ".." {
				out = out[:len(out)-1]
			} else if !abs {
				out = append(out, n)
			}
		default:
			out = append(out, n)
		}
	}
	s := strings.Join(out, "/")
	if abs {
		s = "/" + s
	}
	if s == this.s_ {
		return this
	}
	return &unixPath_{s}
}

func (this *unixPath_) resolve_Path(other Path) Path {
	o := asPath_(other)
	switch {
	case o.isAbsolute() || this.s_ == "":
		return o
	case o.s_ == "":
		return this
	case this.s_ == "/":
		return &unixPath_{"/" + o.s_}
	}
	return &unixPath_{this.s_ + "/" + o.s_}
}

func (this *unixPath_) resolveSibling_Path(other Path) Path {
	parent := this.getParent()
	if parent == nil {
		return asPath_(other)
	}
	return parent.resolve_Path(other)
}

func (this *unixPath_) hasDots_() bool {
	for _, n := range this.names_() {
		if n == "." || n == ".." {
			return true
		}
	}
	return false
}

func (this *unixPath_) relativize(other Path) Path {
	o := asPath_(other)
	if o.s_ == this.s_ {
		return &unixPath_{""}
	}
	if this.isAbsolute() != o.isAbsolute() {
		panic(new_IllegalArgumentException_String("'other' is different type of Path").self())
	}
	if this.s_ == "" {
		return o
	}
	base := this
	if base.hasDots_() || o.hasDots_() {
		// Paths with "." or ".." are compared in normal form; a base that
		// still starts with ".." then has no answer.
		base, o = base.normalize().(*unixPath_), o.normalize().(*unixPath_)
		if base.s_ == "" {
			return o
		}
		if o.s_ == base.s_ {
			return &unixPath_{""}
		}
	}
	a, b := base.names_(), o.names_()
	if o.s_ == "" {
		b = nil
	}
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	out := make([]string, 0, len(a)-i+len(b)-i)
	for _, n := range a[i:] {
		if n == ".." {
			panic(new_IllegalArgumentException_String("Unable to compute relative  path from " + this.s_ + " to " + asPath_(other).s_).self())
		}
		out = append(out, "..")
	}
	out = append(out, b[i:]...)
	return &unixPath_{strings.Join(out, "/")}
}

func (this *unixPath_) toAbsolutePath() Path {
	if this.isAbsolute() {
		return this
	}
	return pathOf_(userDir_()).resolve_Path(this)
}

func (this *unixPath_) toRealPath(opts []*LinkOption) Path {
	abs := this.toAbsolutePath().(*unixPath_).s_
	r, err := realpath_(abs)
	if err != nil {
		panic(nioError_(this.s_, err))
	}
	return &unixPath_{r}
}

func (this *unixPath_) toFile() *File { return new_File(this.s_) }

func (this *unixPath_) toUri() *URI { return this.toFile().toURI() }

func (this *unixPath_) getFileSystem() *FileSystem { return defaultFileSystem_ }

func (this *unixPath_) iterator() Iterator {
	names := this.names_()
	items := make([]any, len(names))
	for i, n := range names {
		items[i] = &unixPath_{n}
	}
	return listOf_(items).iterator()
}

func (this *unixPath_) equals(o any) bool {
	p, ok := o.(*unixPath_)
	return ok && p != nil && p.s_ == this.s_
}

func (this *unixPath_) hashCode() int32 {
	var h int32
	for i := 0; i < len(this.s_); i++ {
		h = 31*h + int32(this.s_[i])
	}
	return h
}

// compareTo compares the bytes of the two paths, then their lengths.
func (this *unixPath_) compareTo(o any) int32 {
	a, b := this.s_, o.(*unixPath_).s_
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return int32(a[i]) - int32(b[i])
		}
	}
	return int32(len(a) - len(b))
}

// ---- FileSystem, PathMatcher

type FileSystems struct{}

type FileSystem struct{}

var defaultFileSystem_ = &FileSystem{}

func FileSystems_getDefault() *FileSystem { return defaultFileSystem_ }

func (this *FileSystem) getPath(first string, more []string) Path { return Path_of(first, more) }

func (this *FileSystem) getSeparator() string { return "/" }

func (this *FileSystem) isOpen() bool { return true }

// close of the default file system is not supported.
func (this *FileSystem) close() {
	panic(new_UnsupportedOperationException_0().self())
}

type regexPathMatcher_ struct {
	re_ *regexp.Regexp
}

func (this *regexPathMatcher_) matches(p Path) bool {
	return this.re_.MatchString(asPath_(p).s_)
}

func (this *FileSystem) getPathMatcher(syntaxAndPattern string) PathMatcher {
	i := strings.IndexByte(syntaxAndPattern, ':')
	if i <= 0 {
		panic(new_IllegalArgumentException_0().self())
	}
	syntax, pattern := syntaxAndPattern[:i], syntaxAndPattern[i+1:]
	var expr string
	switch strings.ToLower(syntax) {
	case "glob":
		expr = globToRegex_(pattern)
	case "regex":
		expr = pattern
	default:
		panic(new_UnsupportedOperationException_String("Syntax '" + syntax + "' not recognized").self())
	}
	re, err := regexp.Compile("^(?:" + expr + ")$")
	if err != nil {
		// java.util.regex.PatternSyntaxException is an IllegalArgumentException.
		panic(new_IllegalArgumentException_String(err.Error()).self())
	}
	return &regexPathMatcher_{re}
}

func globError_(msg string, glob string, i int) any {
	// The message of java.util.regex.PatternSyntaxException.
	return new_IllegalArgumentException_String(msg + " near index " + istr_(int64(i)) + "\n" + glob + "\n" + strings.Repeat(" ", i) + "^").self()
}

// globToRegex_ translates the glob syntax of FileSystem.getPathMatcher.
func globToRegex_(glob string) string {
	var b strings.Builder
	inGroup := false
	rs := []rune(glob)
	lit := func(r rune) { b.WriteString(regexp.QuoteMeta(string(r))) }
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch c {
		case '\\':
			if i+1 == len(rs) {
				panic(globError_("No character to escape", glob, i))
			}
			i++
			lit(rs[i])
		case '/':
			b.WriteByte('/')
		case '[':
			b.WriteString("[")
			i++
			if i < len(rs) && rs[i] == '!' {
				b.WriteString("^/")
				i++
			} else if i < len(rs) && rs[i] == '^' {
				b.WriteString(`\^`)
				i++
			}
			if i < len(rs) && rs[i] == '-' {
				b.WriteString(`\-`)
				i++
			}
			if i < len(rs) && rs[i] == ']' {
				// The JDK builds a regex with an empty class from this, which fails.
				panic(globError_("Unclosed character class", glob, i))
			}
			hasRangeStart := false
			var last rune
			closed := false
			for i < len(rs) {
				c = rs[i]
				if c == ']' {
					closed = true
					break
				}
				if c == '/' {
					panic(globError_("Explicit 'name separator' in class", glob, i))
				}
				if c == '\\' || c == '[' || c == '^' || c == '&' && i+1 < len(rs) && rs[i+1] == '&' {
					b.WriteByte('\\')
				}
				if c == '-' {
					if !hasRangeStart {
						panic(globError_("Invalid range", glob, i))
					}
					i++
					if i >= len(rs) || rs[i] == ']' {
						b.WriteString(`\-`)
						if i < len(rs) {
							closed = true
						}
						break
					}
					c = rs[i]
					if c < last {
						panic(globError_("Invalid range", glob, i-2))
					}
					b.WriteByte('-')
					if c == '\\' || c == '[' || c == ']' || c == '^' {
						b.WriteByte('\\')
					}
					b.WriteRune(c)
					hasRangeStart = false
				} else {
					b.WriteRune(c)
					hasRangeStart = true
					last = c
				}
				i++
			}
			if !closed {
				panic(globError_("Missing ']", glob, i-1))
			}
			b.WriteString("]")
		case '{':
			if inGroup {
				panic(globError_("Cannot nest groups", glob, i))
			}
			b.WriteString("(?:(?:")
			inGroup = true
		case '}':
			if inGroup {
				b.WriteString("))")
				inGroup = false
			} else {
				b.WriteString(`\}`)
			}
		case ',':
			if inGroup {
				b.WriteString(")|(?:")
			} else {
				b.WriteByte(',')
			}
		case '*':
			if i+1 < len(rs) && rs[i+1] == '*' {
				b.WriteString(".*")
				i++
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			lit(c)
		}
	}
	if inGroup {
		panic(globError_("Missing '}", glob, len(rs)-1))
	}
	return b.String()
}

// ---- enums of java.nio.file

type FileVisitResult struct {
	Enum
}

func (this *FileVisitResult) up_Enum() *Enum {
	if this == nil {
		return nil
	}
	return &this.Enum
}

func newFileVisitResult_(name string, ordinal int32) *FileVisitResult {
	e := &FileVisitResult{}
	e.self_ = e
	e.init_Enum(name, ordinal)
	return e
}

var FileVisitResult_CONTINUE = newFileVisitResult_("CONTINUE", 0)
var FileVisitResult_TERMINATE = newFileVisitResult_("TERMINATE", 1)
var FileVisitResult_SKIP_SUBTREE = newFileVisitResult_("SKIP_SUBTREE", 2)
var FileVisitResult_SKIP_SIBLINGS = newFileVisitResult_("SKIP_SIBLINGS", 3)

type FileVisitOption struct {
	Enum
}

func (this *FileVisitOption) up_Enum() *Enum {
	if this == nil {
		return nil
	}
	return &this.Enum
}

var FileVisitOption_FOLLOW_LINKS = func() *FileVisitOption {
	e := &FileVisitOption{}
	e.self_ = e
	e.init_Enum("FOLLOW_LINKS", 0)
	return e
}()

type LinkOption struct {
	Enum
}

func (this *LinkOption) up_Enum() *Enum {
	if this == nil {
		return nil
	}
	return &this.Enum
}

var LinkOption_NOFOLLOW_LINKS = func() *LinkOption {
	e := &LinkOption{}
	e.self_ = e
	e.init_Enum("NOFOLLOW_LINKS", 0)
	return e
}()

type StandardOpenOption struct {
	Enum
}

func (this *StandardOpenOption) up_Enum() *Enum {
	if this == nil {
		return nil
	}
	return &this.Enum
}

func newStandardOpenOption_(name string, ordinal int32) *StandardOpenOption {
	e := &StandardOpenOption{}
	e.self_ = e
	e.init_Enum(name, ordinal)
	return e
}

var StandardOpenOption_READ = newStandardOpenOption_("READ", 0)
var StandardOpenOption_WRITE = newStandardOpenOption_("WRITE", 1)
var StandardOpenOption_APPEND = newStandardOpenOption_("APPEND", 2)
var StandardOpenOption_TRUNCATE_EXISTING = newStandardOpenOption_("TRUNCATE_EXISTING", 3)
var StandardOpenOption_CREATE = newStandardOpenOption_("CREATE", 4)
var StandardOpenOption_CREATE_NEW = newStandardOpenOption_("CREATE_NEW", 5)
var StandardOpenOption_DELETE_ON_CLOSE = newStandardOpenOption_("DELETE_ON_CLOSE", 6)
var StandardOpenOption_SPARSE = newStandardOpenOption_("SPARSE", 7)
var StandardOpenOption_SYNC = newStandardOpenOption_("SYNC", 8)
var StandardOpenOption_DSYNC = newStandardOpenOption_("DSYNC", 9)

// ---- FileTime, BasicFileAttributes

type FileTime struct {
	ns_ int64 // nanoseconds since the epoch
}

func FileTime_fromMillis(ms int64) *FileTime { return &FileTime{ms * 1e6} }

func (this *FileTime) toMillis() int64 { return floorDiv_(this.ns_, 1e6) }

func floorDiv_(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

func (this *FileTime) equals(o any) bool {
	t, ok := o.(*FileTime)
	return ok && t != nil && t.ns_ == this.ns_
}

func (this *FileTime) hashCode() int32 {
	return int32(this.ns_ ^ int64(uint64(this.ns_)>>32))
}

func (this *FileTime) compareTo(o any) int32 {
	t := o.(*FileTime)
	switch {
	case this.ns_ < t.ns_:
		return -1
	case this.ns_ > t.ns_:
		return 1
	}
	return 0
}

// toString is ISO 8601 in UTC, with the fraction of a second only when it is
// not zero.
func (this *FileTime) toString() string {
	t := time.Unix(0, this.ns_).UTC()
	return t.Format("2006-01-02T15:04:05.999999999Z")
}

type fileAttributes_ struct {
	st_ os.FileInfo
}

func (this *fileAttributes_) isDirectory() bool    { return this.st_.IsDir() }
func (this *fileAttributes_) isRegularFile() bool  { return this.st_.Mode().IsRegular() }
func (this *fileAttributes_) isSymbolicLink() bool { return this.st_.Mode()&os.ModeSymlink != 0 }
func (this *fileAttributes_) size() int64          { return this.st_.Size() }

func (this *fileAttributes_) isOther() bool {
	return !this.isDirectory() && !this.isRegularFile() && !this.isSymbolicLink()
}

func (this *fileAttributes_) lastModifiedTime() *FileTime {
	return &FileTime{this.st_.ModTime().UnixNano()}
}

func (this *fileAttributes_) lastAccessTime() *FileTime { return this.lastModifiedTime() }
func (this *fileAttributes_) creationTime() *FileTime   { return this.lastModifiedTime() }
func (this *fileAttributes_) fileKey() any              { return nil }

// ---- java.nio.file.Files

type Files_file struct{}

func noFollow_(opts []*LinkOption) bool {
	for _, o := range opts {
		if o == LinkOption_NOFOLLOW_LINKS {
			return true
		}
	}
	return false
}

func statPath_(p Path, opts []*LinkOption) (os.FileInfo, error) {
	s := asPath_(p).s_
	if s == "" {
		s = "."
	}
	if noFollow_(opts) {
		return os.Lstat(s)
	}
	return os.Stat(s)
}

func Files_file_exists(p Path, opts []*LinkOption) bool {
	_, err := statPath_(p, opts)
	return err == nil
}

func Files_file_isDirectory(p Path, opts []*LinkOption) bool {
	st, err := statPath_(p, opts)
	return err == nil && st.IsDir()
}

func Files_file_isRegularFile(p Path, opts []*LinkOption) bool {
	st, err := statPath_(p, opts)
	return err == nil && st.Mode().IsRegular()
}

func Files_file_size(p Path) int64 {
	st, err := statPath_(p, nil)
	if err != nil {
		panic(nioError_(asPath_(p).s_, err))
	}
	return st.Size()
}

func Files_file_getLastModifiedTime(p Path, opts []*LinkOption) *FileTime {
	st, err := statPath_(p, opts)
	if err != nil {
		panic(nioError_(asPath_(p).s_, err))
	}
	return &FileTime{st.ModTime().UnixNano()}
}

func Files_file_createDirectories(p Path, attrs []FileAttribute) Path {
	s := asPath_(p).s_
	if err := os.MkdirAll(s, 0o777); err != nil {
		if st, e := os.Stat(s); e == nil && !st.IsDir() {
			err = syscall.EEXIST // the path is a file
		}
		panic(nioError_(s, err))
	}
	return p
}

func Files_file_readAllBytes(p Path) []int8 {
	b, err := os.ReadFile(asPath_(p).s_)
	if err != nil {
		panic(nioError_(asPath_(p).s_, err))
	}
	return i8_(b)
}

// Files_file_readString decodes the whole file; malformed input throws a
// CharacterCodingException.
func Files_file_readString(p Path, cs *Charset) string {
	if cs == nil {
		panic(new_NullPointerException_0().self())
	}
	b, err := os.ReadFile(asPath_(p).s_)
	if err != nil {
		panic(nioError_(asPath_(p).s_, err))
	}
	return cs.decodeStrict_(b)
}

func Files_file_newInputStream(p Path, opts []OpenOption) *InputStream {
	s := asPath_(p).s_
	f, err := os.Open(s)
	if err != nil {
		panic(nioError_(s, err))
	}
	return newInputStream_(f, func() { f.Close() })
}

func Files_file_newBufferedReader(p Path, cs *Charset) *BufferedReader {
	if cs == nil {
		panic(new_NullPointerException_0().self())
	}
	return new_BufferedReader(newStrictReader_(Files_file_newInputStream(p, nil), cs).up_Reader())
}

func Files_file_newOutputStream(p Path, opts []OpenOption) *OutputStream {
	s := asPath_(p).s_
	flags := os.O_WRONLY
	if len(opts) == 0 {
		flags |= os.O_CREATE | os.O_TRUNC
	}
	for _, o := range opts {
		switch o {
		case any(StandardOpenOption_APPEND):
			flags |= os.O_APPEND
		case any(StandardOpenOption_TRUNCATE_EXISTING):
			flags |= os.O_TRUNC
		case any(StandardOpenOption_CREATE):
			flags |= os.O_CREATE
		case any(StandardOpenOption_CREATE_NEW):
			flags |= os.O_CREATE | os.O_EXCL
		case any(StandardOpenOption_SYNC), any(StandardOpenOption_DSYNC):
			flags |= os.O_SYNC
		case any(StandardOpenOption_READ):
			panic(new_IllegalArgumentException_String("READ not allowed").self())
		case nil:
			panic(new_NullPointerException_0().self())
		}
	}
	if flags&os.O_APPEND != 0 && flags&os.O_TRUNC != 0 {
		panic(new_IllegalArgumentException_String("APPEND + TRUNCATE_EXISTING not allowed").self())
	}
	f, err := os.OpenFile(s, flags, 0o666)
	if err != nil {
		panic(nioError_(s, err))
	}
	return newFileOutputStream_(f).up_OutputStream()
}

func Files_file_newBufferedWriter(p Path, cs *Charset, opts []OpenOption) *BufferedWriter {
	if cs == nil {
		panic(new_NullPointerException_0().self())
	}
	enc := newEncoder_(cs)
	enc.mode_ = encReport_ // this writer's encoder reports unmappable characters
	return new_BufferedWriter(newOutputStreamWriter_(Files_file_newOutputStream(p, opts), enc).up_Writer())
}

func Files_file_write(p Path, b []int8, opts []OpenOption) Path {
	out := Files_file_newOutputStream(p, opts)
	defer out.close()
	out.write_byteArr(b)
	return p
}

func Files_file_writeString(p Path, s any, cs *Charset, opts []OpenOption) Path {
	return Files_file_write(p, String_getBytes(csString_(s), cs), opts)
}

func Files_file_deleteIfExists(p Path) bool {
	err := os.Remove(asPath_(p).s_)
	if err == nil {
		return true
	}
	if errors.Is(err, syscall.ENOENT) {
		return false
	}
	panic(nioError_(asPath_(p).s_, err))
}

func Files_file_delete(p Path) {
	if err := os.Remove(asPath_(p).s_); err != nil {
		panic(nioError_(asPath_(p).s_, err))
	}
}

// ---- Files.walkFileTree

type SimpleFileVisitor struct {
	self_ any
}

func (this *SimpleFileVisitor) as_SimpleFileVisitor() *SimpleFileVisitor { return this }

func to_SimpleFileVisitor(v any) *SimpleFileVisitor {
	if v == nil {
		return nil
	}
	return v.(interface {
		as_SimpleFileVisitor() *SimpleFileVisitor
	}).as_SimpleFileVisitor()
}

func is_SimpleFileVisitor(v any) bool {
	_, ok := v.(interface {
		as_SimpleFileVisitor() *SimpleFileVisitor
	})
	return ok
}

func (this *SimpleFileVisitor) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *SimpleFileVisitor) init_SimpleFileVisitor() {}

func new_SimpleFileVisitor() *SimpleFileVisitor {
	this := &SimpleFileVisitor{}
	this.self_ = this
	return this
}

func (this *SimpleFileVisitor) preVisitDirectory(dir any, attrs BasicFileAttributes) *FileVisitResult {
	if dir == nil || attrs == nil {
		panic(new_NullPointerException_0().self())
	}
	return FileVisitResult_CONTINUE
}

func (this *SimpleFileVisitor) visitFile(file any, attrs BasicFileAttributes) *FileVisitResult {
	if file == nil || attrs == nil {
		panic(new_NullPointerException_0().self())
	}
	return FileVisitResult_CONTINUE
}

func (this *SimpleFileVisitor) visitFileFailed(file any, exc *IOException) *FileVisitResult {
	if file == nil {
		panic(new_NullPointerException_0().self())
	}
	panic(exc.self())
}

func (this *SimpleFileVisitor) postVisitDirectory(dir any, exc *IOException) *FileVisitResult {
	if dir == nil {
		panic(new_NullPointerException_0().self())
	}
	if exc != nil {
		panic(exc.self())
	}
	return FileVisitResult_CONTINUE
}

type fileVisitor_ interface {
	preVisitDirectory(any, BasicFileAttributes) *FileVisitResult
	visitFile(any, BasicFileAttributes) *FileVisitResult
	visitFileFailed(any, *IOException) *FileVisitResult
	postVisitDirectory(any, *IOException) *FileVisitResult
}

type treeWalk_ struct {
	visitor  fileVisitor_
	follow   bool
	maxDepth int32
	// ancestors are the directories being visited, to find cycles through links.
	ancestors []os.FileInfo
}

func checkResult_(r *FileVisitResult) *FileVisitResult {
	if r == nil {
		panic(new_NullPointerException_0().self())
	}
	return r
}

// walk_ visits one file (a directory with everything below it) and returns
// what the visitor decided last: TERMINATE ends the walk, SKIP_SIBLINGS skips
// the rest of the directory the file is in.
func (w *treeWalk_) walk_(p *unixPath_, depth int32) (r *FileVisitResult) {
	name := p.s_
	if name == "" {
		name = "."
	}
	var st os.FileInfo
	var err error
	if w.follow {
		st, err = os.Stat(name)
		if err != nil {
			st, err = os.Lstat(name) // a link to nowhere is visited as a link
		}
	} else {
		st, err = os.Lstat(name)
	}
	if err != nil {
		return checkResult_(w.visitor.visitFileFailed(p, to_IOException(nioError_(p.s_, err))))
	}
	attrs := &fileAttributes_{st}
	if depth >= w.maxDepth || !st.IsDir() {
		return checkResult_(w.visitor.visitFile(p, attrs))
	}
	if w.follow {
		for _, a := range w.ancestors {
			if os.SameFile(a, st) {
				e := &FileSystemException{}
				e.self_, e.file_ = e, p.s_
				e.init_IOException_String(p.s_)
				return checkResult_(w.visitor.visitFileFailed(p, &e.IOException))
			}
		}
	}
	dir, err := os.Open(name)
	if err != nil {
		return checkResult_(w.visitor.visitFileFailed(p, to_IOException(nioError_(p.s_, err))))
	}
	r = checkResult_(w.visitor.preVisitDirectory(p, attrs))
	if r != FileVisitResult_CONTINUE {
		dir.Close()
		return r
	}
	names, err := dir.Readdirnames(-1)
	dir.Close()
	var failure *IOException
	if err != nil {
		failure = to_IOException(nioError_(p.s_, err))
	}
	w.ancestors = append(w.ancestors, st)
	for _, n := range names {
		r = w.walk_(p.resolve_Path(&unixPath_{n}).(*unixPath_), depth+1)
		if r == FileVisitResult_TERMINATE {
			return r
		}
		if r == FileVisitResult_SKIP_SIBLINGS {
			break
		}
	}
	w.ancestors = w.ancestors[:len(w.ancestors)-1]
	r = checkResult_(w.visitor.postVisitDirectory(p, failure))
	if r == FileVisitResult_SKIP_SUBTREE {
		r = FileVisitResult_CONTINUE // there is no subtree left to skip
	}
	return r
}

func Files_file_walkFileTree(start Path, options Set, maxDepth int32, visitor FileVisitor) Path {
	if maxDepth < 0 {
		panic(new_IllegalArgumentException_String("'maxDepth' is negative").self())
	}
	v, ok := any(visitor).(fileVisitor_)
	if !ok {
		panic(new_NullPointerException_0().self())
	}
	w := &treeWalk_{visitor: v, maxDepth: maxDepth}
	if options != nil {
		w.follow = options.contains(FileVisitOption_FOLLOW_LINKS)
	}
	w.walk_(asPath_(start), 0)
	return start
}

func Files_file_walkFileTree_Path_FileVisitor(start Path, visitor FileVisitor) Path {
	return Files_file_walkFileTree(start, nil, 0x7fffffff, visitor)
}
