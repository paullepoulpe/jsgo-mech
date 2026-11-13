package closure

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strings"
)

// Main is the process entry point. While the port is being brought up it
// understands a few test commands before the real command line:
//
//	mech -parse file.js     parse and print one file
func Main(args []string) (code int) {
	defer func() {
		if r := recover(); r != nil {
			if t, ok := r.(interface{ as_Throwable() *Throwable }); ok {
				// A Java exception that main does not catch: what the JVM prints, with the Go
				// stack where Java has its stack frames, and the JVM's exit status.
				trace := Io_stackTraceText(t.as_Throwable())
				first, rest, _ := strings.Cut(trace, "\n")
				frames := strings.ReplaceAll(strings.TrimSpace(string(debug.Stack())), "\n", "\n\tat ")
				printErr_("Exception in thread \"main\" " + first + "\n\tat " + frames + "\n" + rest)
				code = 1
				return
			}
			fmt.Fprintln(os.Stderr, "panic:", r)
			os.Stderr.Write(debug.Stack())
			code = 70
		}
	}()
	// Measuring hooks, each writing a file when the program exits:
	//   JSGO_MEMSTATS=<file>     runtime.MemStats totals as one line of JSON
	//   JSGO_MEM_PROFILE=<file>  pprof allocation profile (every 4096 bytes sampled; read it with
	//                            `go tool pprof -sample_index=alloc_objects`)
	//   JSGO_CPU_PROFILE=<file>  pprof CPU profile
	if f := os.Getenv("JSGO_MEMSTATS"); f != "" {
		stop := flushStd_
		flushStd_ = func() {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			os.WriteFile(f, []byte(fmt.Sprintf("{\"mallocs\":%d,\"total_alloc\":%d,\"num_gc\":%d,\"pause_total_ns\":%d,\"gc_cpu_fraction\":%g,\"heap_sys\":%d,\"heap_alloc\":%d}\n",
				m.Mallocs, m.TotalAlloc, m.NumGC, m.PauseTotalNs, m.GCCPUFraction, m.HeapSys, m.HeapAlloc)), 0644)
			stop()
		}
	}
	if f := os.Getenv("JSGO_MEM_PROFILE"); f != "" {
		runtime.MemProfileRate = 4096
		stop := flushStd_
		flushStd_ = func() {
			if out, err := os.Create(f); err == nil {
				pprof.Lookup("allocs").WriteTo(out, 0)
				out.Close()
			}
			stop()
		}
	}
	if f := os.Getenv("JSGO_CPU_PROFILE"); f != "" {
		out, err := os.Create(f)
		if err == nil {
			pprof.StartCPUProfile(out)
			stop := flushStd_
			flushStd_ = func() { pprof.StopCPUProfile(); out.Close(); stop() }
		}
	}
	if len(args) == 1 && args[0] == "-init-order" {
		// An instrumented build prints the order to use for static initialization.
		for _, c := range initOrder_ {
			fmt.Println(c)
		}
		for _, f := range initFailures_ {
			fmt.Fprintln(os.Stderr, "init failure:", f)
		}
		return 0
	}
	if os.Getenv("JSGO_DEBUG_INIT") != "" {
		for _, f := range initFailures_ {
			fmt.Fprintln(os.Stderr, "init failure:", f)
		}
	}
	if len(args) == 2 && args[0] == "-parse" {
		src, err := os.ReadFile(args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		compiler := new_Compiler_0()
		compiler.initOptions(new_CompilerOptions())
		n := compiler.parseTestCode(string(src))
		fmt.Println(compiler.toSource_1(n))
		return 0
	}
	CommandLineRunner_main(args)
	flushStd_()
	return 0
}
