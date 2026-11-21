// Command tc type-checks package closure as the post-flip tools load it (rw.Load) and exits 1
// on the first errors. The trial re-flip uses it to check a regenerated patch in seconds.
//
// usage: tc <dir of package closure>
package main

import (
	"os"

	"postflip/rw"
)

func main() {
	rw.Load(os.Args[1])
}
