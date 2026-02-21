package modelmap

// models is the canonical model registry.
// Provider IDs must match what's registered in the provider package.
var models = map[string]ModelInfo{
	// Anthropic Claude
	"claude-sonnet-4-5": {
		ID:        "claude-sonnet-4-5",
		Name:      "Claude Sonnet 4.5",
		Providers: []string{"claude", "copilot", "cursor", "antigravity"},
	},
	"claude-sonnet-4": {
		ID:        "claude-sonnet-4",
		Name:      "Claude Sonnet 4",
		Providers: []string{"claude", "copilot", "cursor", "antigravity"},
	},
	"claude-opus-4": {
		ID:        "claude-opus-4",
		Name:      "Claude Opus 4",
		Providers: []string{"claude", "copilot", "cursor", "antigravity"},
	},
	"claude-haiku-3-5": {
		ID:        "claude-haiku-3-5",
		Name:      "Claude Haiku 3.5",
		Providers: []string{"claude", "copilot", "cursor", "antigravity"},
	},

	// OpenAI GPT
	"gpt-4o": {
		ID:        "gpt-4o",
		Name:      "GPT-4o",
		Providers: []string{"copilot", "cursor", "codex"},
	},
	"gpt-4o-mini": {
		ID:        "gpt-4o-mini",
		Name:      "GPT-4o mini",
		Providers: []string{"copilot", "cursor", "codex"},
	},
	"gpt-4-1": {
		ID:        "gpt-4-1",
		Name:      "GPT-4.1",
		Providers: []string{"copilot", "cursor", "codex"},
	},
	"gpt-4-1-mini": {
		ID:        "gpt-4-1-mini",
		Name:      "GPT-4.1 mini",
		Providers: []string{"copilot", "cursor", "codex"},
	},
	"gpt-4-1-nano": {
		ID:        "gpt-4-1-nano",
		Name:      "GPT-4.1 nano",
		Providers: []string{"copilot", "cursor", "codex"},
	},

	// OpenAI o-series
	"o3": {
		ID:        "o3",
		Name:      "o3",
		Providers: []string{"copilot", "cursor", "codex"},
	},
	"o3-mini": {
		ID:        "o3-mini",
		Name:      "o3-mini",
		Providers: []string{"copilot", "cursor", "codex"},
	},
	"o4-mini": {
		ID:        "o4-mini",
		Name:      "o4-mini",
		Providers: []string{"copilot", "cursor", "codex"},
	},

	// Google Gemini
	"gemini-2-5-pro": {
		ID:        "gemini-2-5-pro",
		Name:      "Gemini 2.5 Pro",
		Providers: []string{"gemini", "copilot", "cursor", "antigravity"},
	},
	"gemini-2-5-flash": {
		ID:        "gemini-2-5-flash",
		Name:      "Gemini 2.5 Flash",
		Providers: []string{"gemini", "copilot", "cursor", "antigravity"},
	},
	"gemini-2-0-flash": {
		ID:        "gemini-2-0-flash",
		Name:      "Gemini 2.0 Flash",
		Providers: []string{"gemini", "copilot", "cursor", "antigravity"},
	},

	// Kimi / Moonshot
	"k2-5": {
		ID:        "k2-5",
		Name:      "Kimi K2.5",
		Providers: []string{"kimi"},
	},

	// Minimax
	"minimax-m2-5": {
		ID:        "minimax-m2-5",
		Name:      "MiniMax M2.5",
		Providers: []string{"minimax"},
	},
	"minimax-m2-1": {
		ID:        "minimax-m2-1",
		Name:      "MiniMax M2.1",
		Providers: []string{"minimax"},
	},
	"minimax-m2": {
		ID:        "minimax-m2",
		Name:      "MiniMax M2",
		Providers: []string{"minimax"},
	},
}

// aliases maps informal/shorthand names to canonical model IDs.
var aliases = map[string]string{
	// Claude shortcuts
	"sonnet":        "claude-sonnet-4",
	"sonnet-4":      "claude-sonnet-4",
	"sonnet-4.5":    "claude-sonnet-4-5",
	"sonnet-4-5":    "claude-sonnet-4-5",
	"sonnet4.5":     "claude-sonnet-4-5",
	"opus":          "claude-opus-4",
	"opus-4":        "claude-opus-4",
	"haiku":         "claude-haiku-3-5",
	"haiku-3.5":     "claude-haiku-3-5",
	"haiku-3-5":     "claude-haiku-3-5",
	"claude-3-5":    "claude-haiku-3-5",
	"claude-sonnet": "claude-sonnet-4",
	"claude-opus":   "claude-opus-4",
	"claude-haiku":  "claude-haiku-3-5",

	// GPT shortcuts
	"4o":           "gpt-4o",
	"4o-mini":      "gpt-4o-mini",
	"gpt4o":        "gpt-4o",
	"gpt-4.1":      "gpt-4-1",
	"gpt4.1":       "gpt-4-1",
	"gpt-4.1-mini": "gpt-4-1-mini",
	"gpt-4.1-nano": "gpt-4-1-nano",

	// o-series
	"o3mini": "o3-mini",
	"o4mini": "o4-mini",

	// Gemini shortcuts
	"gemini":         "gemini-2-5-pro",
	"gemini-pro":     "gemini-2-5-pro",
	"gemini-flash":   "gemini-2-5-flash",
	"gemini-2.5-pro": "gemini-2-5-pro",
	"gemini-2.5":     "gemini-2-5-pro",
	"flash":          "gemini-2-5-flash",

	// Kimi shortcuts
	"kimi": "k2-5",
	"k2.5": "k2-5",

	// Minimax shortcuts
	"m2.5": "minimax-m2-5",
	"m2.1": "minimax-m2-1",
	"m2":   "minimax-m2",
}
