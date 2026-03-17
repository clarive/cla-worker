package main

import (
	"os"

	"github.com/clarive/cla-worker-go/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
