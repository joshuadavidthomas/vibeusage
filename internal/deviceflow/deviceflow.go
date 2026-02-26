// Package deviceflow provides shared utilities for OAuth device flow
// authentication across providers.
package deviceflow

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// PollTimeout is the maximum time to wait for browser authorization.
const PollTimeout = 2 * time.Minute

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

// PollContext returns a context that is cancelled on SIGINT or after
// PollTimeout, whichever comes first. Call cancel to clean up.
func PollContext() (context.Context, context.CancelFunc) {
	sigCtx, sigCancel := signal.NotifyContext(context.Background(), os.Interrupt)
	ctx, cancel := context.WithTimeout(sigCtx, PollTimeout)
	return ctx, func() { cancel(); sigCancel() }
}

// WaitForEnter blocks until the user presses Enter or SIGINT is received.
// Returns nil on Enter, or a context.Canceled error on interrupt.
func WaitForEnter() error {
	sigCtx, sigCancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer sigCancel()

	done := make(chan struct{}, 1)
	go func() {
		_, _ = bufio.NewReader(os.Stdin).ReadBytes('\n')
		done <- struct{}{}
	}()

	select {
	case <-sigCtx.Done():
		return context.Canceled
	case <-done:
		return nil
	}
}

// PollWait sleeps for the given interval or until the context is cancelled.
// Returns false if the context was cancelled (caller should stop polling).
func PollWait(ctx context.Context, interval int) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(time.Duration(interval) * time.Second):
		return true
	}
}
