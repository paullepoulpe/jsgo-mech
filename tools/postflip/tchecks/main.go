// Command tchecks is a post-flip step: the typed Guava preconditions the translator writes
// become inline Go checks.
//
// The typed translator calls variants of the Preconditions functions that take their
// arguments at their own types (rt_conv.go): Preconditions_checkState_boolean_Object_H(c, n)
// with a node handle as the message, boxed only when the check fails; the generic
// Preconditions_checkNotNull_T(v), which compares v with the null of its type and returns it.
// The post-flip steps for the first flip's erased forms (precond, checks, nnchecks) leave
// them. A call used as a statement becomes the function's own body with the arguments in the
// place of the parameters:
//
//	Preconditions_checkState_boolean_Object_H(c, n)  ->  if !c { stateFail_(objstr_(n.box_())) }
//	Preconditions_checkNotNull_T(v)                  ->  if v == nil { panic(new_NullPointerException_0().self()) }
//
// (a check of the constant false is the failure call alone). The same exception with the same
// message, since it is the runtime's code. Order of
// evaluation: Java evaluates the condition and then the message arguments before the call; here
// a message argument is evaluated only when the check fails, so only arguments that can be
// evaluated later without a difference are accepted (rw.Pure: a constant, a variable, a path of
// fields; nothing runs between the condition and them, and they cannot throw). The null of
// checkNotNull_T's argument is nil for a pointer or an interface, nullstr_ for a string, 0 for a
// handle or an enum (an integer type): what `var null T` is in the generic function. Left and
// counted: a call used as an expression (checkNotNull's result), an argument that is not pure,
// a type whose null the tool does not know.
//
// usage: tchecks [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"postflip/rw"
)

func generated(p token.Pos) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.Position(p).Filename), "g_")
}

// nullOf is the spelling of `var null T` for t, or "".
func nullOf(t types.Type) string {
	switch u := t.Underlying().(type) {
	case *types.Pointer, *types.Interface:
		return "nil"
	case *types.Basic:
		if u.Kind() == types.String {
			return "nullstr_"
		}
		if u.Info()&types.IsInteger != 0 {
			return "0"
		}
	}
	return ""
}

func main() {
	dry := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	// The runtime's typed check functions: name -> declaration.
	decls := map[string]*ast.FuncDecl{}
	for _, f := range rw.Files {
		if generated(f.Pos()) {
			continue
		}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Body != nil && strings.HasPrefix(fd.Name.Name, "Preconditions_check") &&
				(strings.HasSuffix(fd.Name.Name, "_H") || strings.HasSuffix(fd.Name.Name, "_T")) {
				decls[fd.Name.Name] = fd
			}
		}
	}
	done, left := map[string]int{}, map[string]int{}
	for _, f := range rw.Files {
		if !generated(f.Pos()) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fun := c.Fun
			if ix, ok := fun.(*ast.IndexExpr); ok {
				fun = ix.X
			} else if ix, ok := fun.(*ast.IndexListExpr); ok {
				fun = ix.X
			}
			id, ok := fun.(*ast.Ident)
			if !ok {
				return true
			}
			fd := decls[id.Name]
			if fd == nil {
				return true
			}
			st, isStmt := rw.Up(c).(*ast.ExprStmt)
			if !isStmt {
				left[id.Name+": used as an expression"]++
				return true
			}
			var params []string
			for _, p := range fd.Type.Params.List {
				for _, nm := range p.Names {
					params = append(params, nm.Name)
				}
			}
			if len(params) != len(c.Args) {
				left[id.Name+": a call with another number of arguments"]++
				return true
			}
			for i, a := range c.Args[1:] {
				if !rw.Pure(a) {
					left[id.Name+": a message argument that is not a constant, a variable or a field path"]++
					_ = i
					return true
				}
			}
			body := strings.TrimSpace(rw.Text(fd.Body))
			body = strings.TrimSuffix(strings.TrimPrefix(body, "{"), "}")
			isT := strings.HasSuffix(id.Name, "_T")
			null := ""
			if isT {
				if !rw.Pure(c.Args[0]) {
					left[id.Name+": the checked value is not a variable or a field path"]++
					return true
				}
				null = nullOf(rw.Info.TypeOf(c.Args[0]))
				if null == "" {
					left[id.Name+": a type whose null the tool does not know"]++
					return true
				}
				if !strings.Contains(body, "var null T\n") || !strings.Contains(body, "return "+params[0]+"\n") && !strings.HasSuffix(strings.TrimSpace(body), "return "+params[0]) {
					left[id.Name+": the runtime function is not of the expected form"]++
					return true
				}
			}
			args := c.Args
			done[id.Name]++
			rw.Rewrite[st] = func() string {
				out := body
				if isT {
					out = strings.Replace(out, "var null T\n", "", 1)
					out = regexp.MustCompile(`\n\s*return `+params[0]+`\s*$`).ReplaceAllString(strings.TrimRight(out, " \t\n"), "")
					out = regexp.MustCompile(`\bnull\b`).ReplaceAllString(out, null)
				}
				for i, p := range params {
					a := rw.Render(args[i])
					if i == 0 && !isT {
						// the condition, used as `!b`
						switch x := rw.Unparen(args[i]).(type) {
						case *ast.Ident, *ast.SelectorExpr, *ast.CallExpr:
							_ = x
						default:
							a = "(" + a + ")"
						}
					}
					out = regexp.MustCompile(`\b`+p+`\b`).ReplaceAllLiteralString(out, a)
				}
				out = strings.TrimSpace(out)
				// a check of the constant false (what precond and the translator write in a
				// failing branch) is the failure itself
				if m := regexp.MustCompile(`(?s)^if !false \{\n\s*(.*?)\n\s*\}$`).FindStringSubmatch(out); m != nil {
					out = m[1]
				}
				return out
			}
			return false
		})
	}
	total := 0
	for _, k := range sorted(done) {
		total += done[k]
		fmt.Fprintf(os.Stderr, "  %d %s\n", done[k], k)
	}
	fmt.Fprintf(os.Stderr, "tchecks: %d typed precondition calls are inline checks\n", total)
	for _, k := range sorted(left) {
		fmt.Fprintf(os.Stderr, "left: %d %s\n", left[k], k)
	}
	if !dry {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.SaveChecked())
	}
}

func sorted(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
