package main

import (
	"os"

	"github.com/joshuadavidthomas/vibeusage/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
