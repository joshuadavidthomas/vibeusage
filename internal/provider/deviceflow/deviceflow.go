// Package deviceflow provides shared utilities for OAuth device flow
// authentication across providers.
package deviceflow

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
)

// OpenBrowser tries to open a URL in the default browser.
func OpenBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	_ = cmd.Start()
}

// WriteSuccess writes the standard authentication success message.
func WriteSuccess(w io.Writer) {
	_, _ = fmt.Fprintln(w, "✓ Authentication successful!")
}

// WriteTimeout writes the standard timeout message.
func WriteTimeout(w io.Writer) {
	_, _ = fmt.Fprintln(w, "⏱ Timeout waiting for authorization.")
}

// WriteDenied writes the standard authorization denied message.
func WriteDenied(w io.Writer) {
	_, _ = fmt.Fprintln(w, "✗ Authorization denied by user.")
}

// WriteExpired writes the standard device code expired message.
func WriteExpired(w io.Writer) {
	_, _ = fmt.Fprintln(w, "✗ Device code expired.")
}

// WriteWaiting writes the standard "waiting for browser" message.
func WriteWaiting(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Waiting for browser authorization...")
}

// WriteOpening writes the "Opening <url>" message and opens the browser.
func WriteOpening(w io.Writer, url string) {
	_, _ = fmt.Fprintf(w, "Opening %s\n", url)
	OpenBrowser(url)
}
