// Command mech runs the translated Closure Compiler.
package main

import (
	"os"

	"jsgomech/closure"
)

func main() {
	os.Exit(closure.Main(os.Args[1:]))
}
