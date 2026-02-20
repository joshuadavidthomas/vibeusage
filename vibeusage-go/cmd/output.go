package cmd

import (
	"fmt"
	"io"
	"os"
)

// outWriter is the writer used for all command output.
// Tests can replace this to capture output.
var outWriter io.Writer = os.Stdout

// out prints formatted output to the configured writer.
func out(format string, a ...any) {
	fmt.Fprintf(outWriter, format, a...)
}

// outln prints a line to the configured writer.
func outln(a ...any) {
	fmt.Fprintln(outWriter, a...)
}
