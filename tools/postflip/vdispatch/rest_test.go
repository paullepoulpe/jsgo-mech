package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestRest runs vdispatch -rest on a copy of testdata/rest/in (a small package closure:
// g_cases.go holds one hierarchy per rule, rt_stub.go a root of the runtime) and compares what
// it prints with testdata/rest/want.txt and what it writes with testdata/rest/want. UPDATE=1
// rewrites the expectations.
//
// The cases: Gen (generic, dispatching through vt_Gen_: self_ takes the field's type, the field
// goes, the cast is one assertion to Gen_as[T], as_Gen_ is deleted) with its subclass GenSub,
// whose own vt_GenSub_ stays; Ref (generic, no vt field: self_ is Ref_as[T]); Tree (Tree_as is
// declared: used as it is); Reporter (a concrete assertion and a comparison of self_: typed
// Reporter_as, declared here); Copied (self_ copied into a variable), Unpaired (a constructor
// sets self_ and not the vt field) and Foreign (self_ set to an object of another class) are
// left alone; Listed's cast is left (an interface lists as_Listed_) and its root typed; the
// enum Fmt's anonymous interface is named Fmt_as; RtRoot is not looked at.
func TestRest(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "vdispatch")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	dir := filepath.Join(t.TempDir(), "closure")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ins, _ := filepath.Glob("testdata/rest/in/*.go")
	for _, in := range ins {
		b, err := os.ReadFile(in)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(in)), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run := func() []byte {
		var stderr bytes.Buffer
		cmd := exec.Command(bin, "-rest", dir)
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("vdispatch -rest: %v\n%s", err, stderr.String())
		}
		// the step formats what the tool wrote (`run t vdispatch -rest && f`)
		if out, err := exec.Command("gofmt", "-w", dir).CombinedOutput(); err != nil {
			t.Fatalf("gofmt of the output: %v\n%s", err, out)
		}
		// the tool prints positions relative to the directory it is given
		return bytes.ReplaceAll(stderr.Bytes(), []byte(dir+string(filepath.Separator)), nil)
	}
	update := os.Getenv("UPDATE") != ""
	check := func(want string, got []byte) {
		if update {
			if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(want, got, 0o644); err != nil {
				t.Fatal(err)
			}
			return
		}
		w, err := os.ReadFile(want)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(w, got) {
			t.Errorf("%s differs; got:\n%s", want, got)
		}
	}
	read := func() map[string][]byte {
		m := map[string][]byte{}
		for _, in := range ins {
			got, err := os.ReadFile(filepath.Join(dir, filepath.Base(in)))
			if err != nil {
				t.Fatal(err)
			}
			m[filepath.Base(in)] = got
		}
		return m
	}
	check("testdata/rest/want.txt", run())
	first := read()
	for name, got := range first {
		// the expectations are not .go files: they are no package of this module
		check(filepath.Join("testdata/rest/want", name+".txt"), got)
	}
	// the output is a package that compiles
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module closure\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	vet := exec.Command("go", "vet", ".")
	vet.Dir = dir
	if out, err := vet.CombinedOutput(); err != nil {
		t.Errorf("go vet of the output: %v\n%s", err, out)
	}
	os.Remove(filepath.Join(dir, "go.mod"))
	// a second run converts nothing more and writes nothing
	check("testdata/rest/want2.txt", run())
	for name, got := range read() {
		if !bytes.Equal(got, first[name]) {
			t.Errorf("the second run changed %s", name)
		}
	}
}
