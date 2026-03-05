// vibeshots generates mock CLI output for README screenshots.
//
// This is a development-only tool — it is not distributed to users.
// Each subcommand renders a specific "scene" using the real display
// functions with curated mock data, writing ANSI-styled output to stdout.
// Pipe the output through `freeze` to produce SVG images.
//
// Usage:
//
//	go run ./cmd/vibeshots <scene>
//	go run ./cmd/vibeshots --list
package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	// Register all providers so display.DisplayName resolves correctly.
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/amp"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/antigravity"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/claude"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/codex"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/copilot"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/cursor"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/gemini"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/kimicode"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/minimax"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/openrouter"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/warp"
	_ "github.com/joshuadavidthomas/vibeusage/internal/provider/zai"
)

// promptStyle renders the "$ vibeusage ..." command line shown above each output.
var promptStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("7"))

func printPrompt(cmd string) {
	fmt.Println(promptStyle.Render("$ " + cmd))
	fmt.Println()
}

var scenes = map[string]func(){
	"hero":              sceneHero,
	"auth":              sceneAuth,
	"dashboard":         sceneDashboard,
	"usage-single":      sceneUsageSingle,
	"statusline":        sceneStatusline,
	"statusline-short":  sceneStatuslineShort,
	"statusline-single": sceneStatuslineSingle,
	"route-model":       sceneRouteModel,
	"route-role":        sceneRouteRole,
	"status":            sceneStatus,
}

var sceneOrder = []string{
	"hero",
	"auth",
	"dashboard",
	"usage-single",
	"statusline",
	"statusline-short",
	"statusline-single",
	"route-model",
	"route-role",
	"status",
}

func main() {
	// Force true-color output even when piping to ensure lipgloss
	// emits 24-bit ANSI codes that freeze can capture faithfully.
	lipgloss.SetColorProfile(termenv.TrueColor)

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: vibeshots <scene>\n")
		fmt.Fprintf(os.Stderr, "       vibeshots --list\n")
		os.Exit(1)
	}

	arg := os.Args[1]

	if arg == "--list" {
		for _, name := range sceneOrder {
			fmt.Println(name)
		}
		return
	}

	fn, ok := scenes[arg]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown scene: %q\n", arg)
		fmt.Fprintf(os.Stderr, "available scenes:\n")
		for _, name := range sceneOrder {
			fmt.Fprintf(os.Stderr, "  %s\n", name)
		}
		os.Exit(1)
	}

	fn()
}
