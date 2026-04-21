package display

import "testing"

func TestRenderUpdateCheckHeader_NoColor(t *testing.T) {
	got := RenderUpdateCheckHeader("v1.0.0", "v1.1.0", true)
	want := "↑ Update available: v1.0.0 → v1.1.0"
	if got != want {
		t.Fatalf("RenderUpdateCheckHeader() = %q, want %q", got, want)
	}
}

func TestRenderUpdateCheckHeader_WithColor(t *testing.T) {
	got := stripANSI(RenderUpdateCheckHeader("v1.0.0", "v1.1.0", false))
	want := "↑ Update available: v1.0.0 → v1.1.0"
	if got != want {
		t.Fatalf("RenderUpdateCheckHeader() stripped = %q, want %q", got, want)
	}
}
