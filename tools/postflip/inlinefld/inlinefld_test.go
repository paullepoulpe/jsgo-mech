package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestInlinefld runs inlinefld on a copy of testdata/in (a small package closure: g_cases.go
// holds the cases, rt_stub.go a stub of the runtime's typed collections) and compares what it
// prints with testdata/want.txt and what it writes with testdata/want. UPDATE=1 rewrites the
// expectations.
//
// Kept by value: Node's two lists (newTListN_; reads as values become &x.f, x.f.add(v) stays),
// Gen's map and set (generic; explicit type arguments kept), Base's list (read through a *Sub)
// and Sub's (its constructor runs Base's first, which lets nothing escape). Refused, one reason
// each: Twice (assigned again), Setter (assigned outside finit_), Copied (not a constructor of a
// typed collection), Cond (under a condition), NilTest, Compared, Addr, Keyed (a composite
// literal's key), ByValue (an object copied), Elsewhere (a literal outside new_), Escape and
// SuperEscape (the object escapes before finit_), Cloned (Object_clone), Deleg (a delegating
// constructor), Reinit (a constructor called by a method), ZeroRead, RtRead (the runtime reads
// it) and Beside's a (declared beside a refused field).
func TestInlinefld(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "inlinefld")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	dir := filepath.Join(t.TempDir(), "closure")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ins, _ := filepath.Glob("testdata/in/*.go")
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
		cmd := exec.Command(bin, "-v", dir)
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("inlinefld: %v\n%s", err, stderr.String())
		}
		// the step formats what the tool wrote (`run t inlinefld && f`)
		if out, err := exec.Command("gofmt", "-w", dir).CombinedOutput(); err != nil {
			t.Fatalf("gofmt of the output: %v\n%s", err, out)
		}
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
	check("testdata/want.txt", run())
	first := read()
	for name, got := range first {
		check(filepath.Join("testdata/want", name+".txt"), got)
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
	// a second run keeps nothing more by value and writes nothing
	check("testdata/want2.txt", run())
	for name, got := range read() {
		if !bytes.Equal(got, first[name]) {
			t.Errorf("the second run changed %s", name)
		}
	}
}
