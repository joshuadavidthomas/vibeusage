package spinner

import (
	tea "github.com/charmbracelet/bubbletea"
)

// ShouldShow returns true if the spinner should be displayed.
// The spinner is hidden for quiet mode, JSON output, or non-TTY (piped) output.
func ShouldShow(quiet, json, nonTTY bool) bool {
	return !quiet && !json && !nonTTY
}

// Run starts a spinner tracking the given provider IDs.
// It calls fetchFn, passing a callback that fetchFn should invoke
// when each provider completes. Run blocks until all providers finish.
func Run(providerIDs []string, fetchFn func(onComplete func(CompletionInfo))) error {
	if len(providerIDs) == 0 {
		fetchFn(func(CompletionInfo) {})
		return nil
	}

	m := newModel(providerIDs)
	p := tea.NewProgram(m)

	done := make(chan struct{})
	go func() {
		fetchFn(func(info CompletionInfo) {
			p.Send(completionMsg(info))
		})
		close(done)
	}()

	_, err := p.Run()
	<-done
	return err
}
