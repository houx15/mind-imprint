// Command syncrubric mirrors the canonical rubric JSON files from
// packages/contracts/src into internal/rubric so they can be embedded.
// It is the ONLY writer of the mirrors; never hand-edit them.
package main

import (
	"fmt"
	"os"
)

const canonicalDir = "../../packages/contracts/src"
const mirrorDir = "internal/rubric"

var files = []string{"dualaxis.json"}

func main() {
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "syncrubric:", err)
		os.Exit(1)
	}
	for _, f := range files {
		canonical := canonicalDir + "/" + f
		mirror := mirrorDir + "/" + f
		data, err := os.ReadFile(canonical)
		if err != nil {
			fmt.Fprintln(os.Stderr, "syncrubric:", err)
			os.Exit(1)
		}
		if err := os.WriteFile(mirror, data, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "syncrubric:", err)
			os.Exit(1)
		}
		fmt.Println("syncrubric: mirrored", f)
	}
}
