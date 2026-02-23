package cursor

import (
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
)

func TestFetchStrategies_InjectsHTTPTimeout(t *testing.T) {
	c := Cursor{}
	strategies := c.FetchStrategies()

	if len(strategies) == 0 {
		t.Fatal("expected at least one strategy")
	}

	expectedTimeout := config.Get().Fetch.Timeout

	ws, ok := strategies[0].(*WebStrategy)
	if !ok {
		t.Fatalf("expected *WebStrategy, got %T", strategies[0])
	}
	if ws.HTTPTimeout != expectedTimeout {
		t.Errorf("HTTPTimeout = %v, want %v", ws.HTTPTimeout, expectedTimeout)
	}
}
