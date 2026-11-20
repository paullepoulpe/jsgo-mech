package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCases runs the tool on a copy of testdata/in (a small package closure: g_cases.go holds
// one class per rule, rt_stub.go the runtime they use) and compares what it prints with
// testdata/want.txt and what it writes with testdata/want. UPDATE=1 rewrites the expectations.
//
// The cases: Pos and Span qualify (Span is measured with its two Pos as values; Holder.last is
// compared with nil and stays a *Pos that takes a ref_() copy; the typed list of Pos becomes a
// list of values); Ident is compared by identity; Boxed is stored as an Object; Keyed is a map
// key; Maybe is passed as nil (its field stays a pointer: no field would hold it); Late's field
// is set after construction; Counter is not immutable.
func TestCases(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "valplain")
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
	var stderr bytes.Buffer
	cmd := exec.Command(bin, "-v", "-copies", "4", dir)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("valplain: %v\n%s", err, stderr.String())
	}
	// the step formats what the tool wrote (`run t valplain && f`)
	if out, err := exec.Command("gofmt", "-w", dir).CombinedOutput(); err != nil {
		t.Fatalf("gofmt of the output: %v\n%s", err, out)
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
	check("testdata/want.txt", stderr.Bytes())
	for _, in := range ins {
		got, err := os.ReadFile(filepath.Join(dir, filepath.Base(in)))
		if err != nil {
			t.Fatal(err)
		}
		// the expectations are not .go files: they are no package of this module
		check(filepath.Join("testdata/want", filepath.Base(in)+".txt"), got)
	}
	// a second run finds nothing to convert
	stderr.Reset()
	cmd = exec.Command(bin, dir)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("second run: %v\n%s", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "valplain: nothing written") {
		t.Errorf("the second run wrote something:\n%s", stderr.String())
	}
	// the converted package still builds as Go
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module closure\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	vet := exec.Command("go", "vet", ".")
	vet.Dir = dir
	if out, err := vet.CombinedOutput(); err != nil {
		t.Errorf("go vet of the output: %v\n%s", err, out)
	}
}
