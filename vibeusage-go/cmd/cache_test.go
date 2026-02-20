package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestCacheShowCmd_HasTableBorders(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmp)
	reloadConfig()

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldNoColor := noColor
	noColor = false
	defer func() { noColor = oldNoColor }()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	if err := cacheShowCmd.RunE(cacheShowCmd, nil); err != nil {
		t.Fatalf("cacheShowCmd error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "╭") {
		t.Errorf("expected lipgloss rounded border in cache show, got:\n%s", output)
	}
}

func TestCacheShowCmd_ContainsHeaders(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmp)
	reloadConfig()

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldNoColor := noColor
	noColor = true
	defer func() { noColor = oldNoColor }()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	if err := cacheShowCmd.RunE(cacheShowCmd, nil); err != nil {
		t.Fatalf("cacheShowCmd error: %v", err)
	}

	output := buf.String()
	for _, header := range []string{"Provider", "Snapshot", "Org ID", "Age"} {
		if !strings.Contains(output, header) {
			t.Errorf("output missing header %q\n\nGot:\n%s", header, output)
		}
	}
}

func TestCacheShowCmd_QuietMode(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmp)
	reloadConfig()

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	if err := cacheShowCmd.RunE(cacheShowCmd, nil); err != nil {
		t.Fatalf("cacheShowCmd error: %v", err)
	}

	output := buf.String()

	if strings.Contains(output, "╭") {
		t.Error("quiet mode should not use table borders")
	}
}

func TestCacheShowCmd_ShowsCacheDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmp)
	reloadConfig()

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldNoColor := noColor
	noColor = true
	defer func() { noColor = oldNoColor }()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	if err := cacheShowCmd.RunE(cacheShowCmd, nil); err != nil {
		t.Fatalf("cacheShowCmd error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Cache directory:") {
		t.Errorf("expected cache directory path in output, got:\n%s", output)
	}
}
