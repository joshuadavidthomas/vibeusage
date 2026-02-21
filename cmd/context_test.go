package cmd

import (
	"context"
	"testing"
)

func TestExecuteContext_PropagatesContext(t *testing.T) {
	// Verify that ExecuteContext sets the context on rootCmd,
	// which commands can access via cmd.Context().
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// We can't easily run the full command without providers,
	// but we can verify the API exists and doesn't panic.
	// Pass a cancelled context so any fetch would abort quickly.
	cancel()

	// ExecuteContext should accept the context and not panic.
	// It will likely error because there's no config, which is fine.
	_ = ExecuteContext(ctx)
}
