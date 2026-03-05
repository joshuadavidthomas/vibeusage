package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// tableWidth is the terminal width used for table rendering in screenshots.
// Wide enough that provider names and columns don't wrap. The role route
// table is the widest (Model + Provider + Usage bar + Headroom + Cost +
// Period + Resets In + Plan), so this must accommodate that.
const tableWidth = 120

func sceneHero() {
	// The hero image shows the full dashboard — same as sceneDashboard
	// but without the prompt line, for use as a banner at the top of the README.
	snapshots := []models.UsageSnapshot{
		mockClaudeSnapshot(),
		mockCodexSnapshot(),
		mockCopilotSnapshot(),
	}

	cw := display.GlobalPeriodColWidths(snapshots)

	for i, snap := range snapshots {
		if i > 0 {
			fmt.Println()
		}
		fmt.Println(display.RenderProviderPanel(snap, false, cw))
	}
}

func sceneAuth() {
	printPrompt("vibeusage auth")

	bar := lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Render("┃")
	title := lipgloss.NewStyle().Bold(true)
	desc := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursor := lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	selected := lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	detected := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

	fmt.Println(bar + " " + title.Render("Choose providers to set up"))
	fmt.Println(bar + " " + desc.Render("Space to select, Enter to confirm"))

	type item struct {
		id          string
		label       string
		description string
		detection   string
		isSelected  bool
		isCursor    bool
	}

	items := []item{
		{"claude", "claude", "Anthropic's Claude AI assistant (claude.ai)", "detected: provider CLI", true, true},
		{"codex", "codex", "OpenAI's Codex/ChatGPT (platform.openai.com)", "detected: provider CLI", true, false},
		{"copilot", "copilot", "GitHub Copilot (github.com)", "", false, false},
		{"cursor", "cursor", "Cursor AI code editor (cursor.com)", "", false, false},
	}

	for _, it := range items {
		prefix := "  "
		if it.isCursor {
			prefix = cursor.Render("> ")
		}

		check := "[ ]"
		if it.isSelected {
			check = selected.Render("[x]")
		}

		line := bar + " " + prefix + check + " " + it.label
		if it.description != "" {
			line += " " + desc.Render("— "+it.description)
		}
		if it.detection != "" {
			line += " " + detected.Render("["+it.detection+"]")
		}

		fmt.Println(line)
	}

	fmt.Println(bar + " " + desc.Render("  ..."))
}

func sceneDashboard() {
	printPrompt("vibeusage")

	snapshots := []models.UsageSnapshot{
		mockClaudeSnapshot(),
		mockCodexSnapshot(),
		mockCopilotSnapshot(),
		mockCursorSnapshot(),
		mockGeminiSnapshot(),
	}

	cw := display.GlobalPeriodColWidths(snapshots)

	for i, snap := range snapshots {
		if i > 0 {
			fmt.Println()
		}
		fmt.Println(display.RenderProviderPanel(snap, false, cw))
	}
}

func sceneUsageSingle() {
	printPrompt("vibeusage usage claude")

	snap := mockClaudeDetailSnapshot()
	opts := display.DetailOptions{
		Status: snap.Status,
	}
	fmt.Println(display.RenderSingleProvider(snap, false, opts))
}

func sceneStatusline() {
	printPrompt("vibeusage statusline")

	outcomes := mockStatuslineOutcomes()
	_ = display.RenderStatusline(os.Stdout, outcomes, display.StatuslineOptions{
		Mode: display.StatuslineModePretty,
	})
}

func sceneStatuslineShort() {
	printPrompt("vibeusage statusline --short")

	outcomes := mockStatuslineOutcomes()
	_ = display.RenderStatusline(os.Stdout, outcomes, display.StatuslineOptions{
		Mode: display.StatuslineModeShort,
	})
}

func sceneStatuslineSingle() {
	printPrompt("vibeusage statusline -p claude")

	outcomes := mockStatuslineSingleOutcome()
	_ = display.RenderStatusline(os.Stdout, outcomes, display.StatuslineOptions{
		Mode: display.StatuslineModePretty,
	})
}

func sceneRouteModel() {
	printPrompt("vibeusage route claude-opus-4-6")

	rec := mockRouteModelRecommendation()

	fmt.Println(titleStyle.Render("Route: " + rec.ModelName))
	fmt.Println()

	ft := display.FormatRecommendationRows(rec, display.RouteRenderBar, display.RouteFormatReset)
	fmt.Println(display.RenderFormattedTable(ft, display.TableOptions{Width: tableWidth}))
}

func sceneRouteRole() {
	printPrompt("vibeusage route --role coding")

	rec := mockRouteRoleRecommendation()

	fmt.Println(titleStyle.Render("Route: " + rec.Role + " (role)"))
	fmt.Println()

	ft := display.FormatRoleRecommendationRows(rec, display.RouteRenderBar, display.RouteFormatReset)
	fmt.Println(display.RenderFormattedTable(ft, display.TableOptions{Width: tableWidth}))
}

func sceneStatus() {
	printPrompt("vibeusage status")

	statuses := mockProviderStatuses()
	headers, rows := display.FormatStatusRows(statuses, false)
	fmt.Println(display.NewTableWithOptions(
		headers,
		rows,
		display.TableOptions{Title: "Provider Status", Width: 80},
	))
}

// titleStyle is used for route headers — replicates what the CLI does.
var titleStyle = lipgloss.NewStyle().Bold(true)
