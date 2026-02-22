package display

import (
	"os"

	"github.com/charmbracelet/x/term"
)

// TerminalWidth returns the current terminal width, or 80 as a fallback.
func TerminalWidth() int {
	w, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || w <= 0 {
		return 80
	}
	return w
}
