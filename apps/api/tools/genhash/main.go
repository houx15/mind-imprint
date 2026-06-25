// Command genhash prints an argon2id PHC hash for a password, for pasting into
// the seed migration. Usage: go run ./tools/genhash <password>
package main

import (
	"fmt"
	"os"

	"mindimprint/api/internal/auth"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: genhash <password>")
		os.Exit(2)
	}
	phc, err := auth.HashPassword(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(phc)
}
