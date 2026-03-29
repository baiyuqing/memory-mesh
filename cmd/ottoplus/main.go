// Command ottoplus is the CLI for listing available blocks and
// validating composition files against the block registry.
package main

import (
	"fmt"
	"os"

	"github.com/baiyuqing/ottoplus/src/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
