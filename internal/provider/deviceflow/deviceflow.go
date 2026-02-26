// Package deviceflow provides shared utilities for OAuth device flow
// authentication across providers.
package deviceflow

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"

	"github.com/charmbracelet/lipgloss"
)

var (
	green  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	red    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	dim    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	bold   = lipgloss.NewStyle().Bold(true)
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
	_, _ = fmt.Fprintln(w, green.Render("✓ Authentication successful!"))
}

// WriteTimeout writes the standard timeout message.
func WriteTimeout(w io.Writer) {
	_, _ = fmt.Fprintln(w, yellow.Render("⏱ Timeout waiting for authorization."))
}

// WriteDenied writes the standard authorization denied message.
func WriteDenied(w io.Writer) {
	_, _ = fmt.Fprintln(w, red.Render("✗ Authorization denied by user."))
}

// WriteExpired writes the standard device code expired message.
func WriteExpired(w io.Writer) {
	_, _ = fmt.Fprintln(w, red.Render("✗ Device code expired."))
}

// WriteWaiting writes the standard "waiting for browser" message.
func WriteWaiting(w io.Writer) {
	_, _ = fmt.Fprintln(w, dim.Render("Waiting for browser authorization..."))
}

// WriteOpening writes the "Opening <url>" message and opens the browser.
func WriteOpening(w io.Writer, url string) {
	_, _ = fmt.Fprintf(w, "Opening %s\n", bold.Render(url))
	OpenBrowser(url)
}

// InterruptContext returns a context that is cancelled on SIGINT/SIGTERM.
// Call the returned cancel function to clean up signal handling.
func InterruptContext() (context.Context, context.CancelFunc) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	return ctx, cancel
}
