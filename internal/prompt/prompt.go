package prompt

import (
	"github.com/charmbracelet/huh"
)

// InputConfig holds configuration for a text input prompt.
type InputConfig struct {
	Title       string
	Placeholder string
	Validate    func(string) error
}

// ConfirmConfig holds configuration for a yes/no confirmation prompt.
type ConfirmConfig struct {
	Title       string
	Description string
	Affirmative string
	Negative    string
}

// SelectOption represents a single option in a multi-select.
type SelectOption struct {
	Label    string
	Value    string
	Selected bool
}

// MultiSelectConfig holds configuration for a multi-select prompt.
type MultiSelectConfig struct {
	Title       string
	Description string
	Options     []SelectOption
	Validate    func([]string) error
}

// Prompter defines the interface for interactive user prompts.
// This allows swapping the real huh implementation for a mock in tests.
type Prompter interface {
	Input(cfg InputConfig) (string, error)
	Confirm(cfg ConfirmConfig) (bool, error)
	MultiSelect(cfg MultiSelectConfig) ([]string, error)
}

// Default is the package-level prompter used by commands.
// In production this is a Huh instance; tests can swap it with a Mock.
var Default Prompter = &Huh{}

// SetDefault replaces the package-level prompter.
func SetDefault(p Prompter) {
	Default = p
}

// Huh implements Prompter using charmbracelet/huh forms.
type Huh struct{}

func (h *Huh) Input(cfg InputConfig) (string, error) {
	var value string
	input := huh.NewInput().
		Title(cfg.Title).
		Value(&value)

	if cfg.Placeholder != "" {
		input.Placeholder(cfg.Placeholder)
	}
	if cfg.Validate != nil {
		input.Validate(cfg.Validate)
	}

	err := huh.NewForm(huh.NewGroup(input)).Run()
	return value, err
}

func (h *Huh) Confirm(cfg ConfirmConfig) (bool, error) {
	var value bool
	confirm := huh.NewConfirm().
		Title(cfg.Title).
		Value(&value)

	if cfg.Description != "" {
		confirm.Description(cfg.Description)
	}
	if cfg.Affirmative != "" {
		confirm.Affirmative(cfg.Affirmative)
	}
	if cfg.Negative != "" {
		confirm.Negative(cfg.Negative)
	}

	err := huh.NewForm(huh.NewGroup(confirm)).Run()
	return value, err
}

func (h *Huh) MultiSelect(cfg MultiSelectConfig) ([]string, error) {
	var selected []string
	options := make([]huh.Option[string], len(cfg.Options))
	for i, opt := range cfg.Options {
		options[i] = huh.NewOption(opt.Label, opt.Value).Selected(opt.Selected)
	}

	ms := huh.NewMultiSelect[string]().
		Title(cfg.Title).
		Options(options...).
		Value(&selected)

	if cfg.Description != "" {
		ms.Description(cfg.Description)
	}
	if cfg.Validate != nil {
		validate := cfg.Validate
		ms.Validate(func(v []string) error {
			return validate(v)
		})
	}

	err := huh.NewForm(huh.NewGroup(ms)).Run()
	return selected, err
}
