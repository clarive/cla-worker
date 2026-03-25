package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/clarive/cla-worker-go/cmd"
	"github.com/clarive/cla-worker-go/internal/version"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "cla-worker %s crashed: %v\n%s\n", version.String(), r, debug.Stack())
			os.Exit(1)
		}
	}()

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
