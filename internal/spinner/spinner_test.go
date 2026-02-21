package spinner

import "testing"

func TestShouldShowSpinner(t *testing.T) {
	tests := []struct {
		name   string
		quiet  bool
		json   bool
		nonTTY bool
		want   bool
	}{
		{"interactive", false, false, false, true},
		{"quiet mode", true, false, false, false},
		{"json mode", false, true, false, false},
		{"both quiet and json", true, true, false, false},
		{"non-TTY (piped)", false, false, true, false},
		{"quiet and non-TTY", true, false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldShow(tt.quiet, tt.json, tt.nonTTY)
			if got != tt.want {
				t.Errorf("ShouldShow(quiet=%v, json=%v, nonTTY=%v) = %v, want %v",
					tt.quiet, tt.json, tt.nonTTY, got, tt.want)
			}
		})
	}
}

func TestRunEmptyProviders(t *testing.T) {
	called := false
	err := Run([]string{}, func(onComplete func(CompletionInfo)) {
		called = true
	})
	if err != nil {
		t.Errorf("Run with empty providers returned error: %v", err)
	}
	if !called {
		t.Error("expected fetchFn to be called even with empty providers")
	}
}
