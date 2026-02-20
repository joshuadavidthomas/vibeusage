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
	m := newModel(providerIDs)
	p := tea.NewProgram(m)

	go func() {
		fetchFn(func(info CompletionInfo) {
			p.Send(completionMsg(info))
		})
	}()

	_, err := p.Run()
	return err
}
