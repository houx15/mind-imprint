// Command syncrubric mirrors the canonical CT rubric JSON from
// packages/contracts/src/ct-rubric.json into internal/rubric so it can be
// embedded. It is the ONLY writer of the mirror; never hand-edit it.
package main

import (
	"fmt"
	"os"
)

const canonical = "../../packages/contracts/src/ct-rubric.json"
const mirror = "internal/rubric/ct-rubric.json"

func main() {
	data, err := os.ReadFile(canonical)
	if err != nil {
		fmt.Fprintln(os.Stderr, "syncrubric:", err)
		os.Exit(1)
	}
	if err := os.MkdirAll("internal/rubric", 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "syncrubric:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(mirror, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "syncrubric:", err)
		os.Exit(1)
	}
	fmt.Println("syncrubric: mirrored ct-rubric.json")
}
