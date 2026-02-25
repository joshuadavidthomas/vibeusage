package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/testenv"
)

func TestCacheShowCmd_HasTableBorders(t *testing.T) {
	tmp := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmp)
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
	testenv.ApplySameDir(t.Setenv, tmp)
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
	testenv.ApplySameDir(t.Setenv, tmp)
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
	testenv.ApplySameDir(t.Setenv, tmp)
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

// JSON output tests

func TestCacheClearJSON_UsesTypedStruct(t *testing.T) {
	tmp := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmp)
	reloadConfig()

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	if err := cacheClearCmd.RunE(cacheClearCmd, nil); err != nil {
		t.Fatalf("cache clear error: %v", err)
	}

	var result display.ActionResultJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("cache clear JSON should unmarshal into ActionResultJSON: %v\nOutput: %s", err, buf.String())
	}

	if !result.Success {
		t.Error("success should be true")
	}
	if result.Message == "" {
		t.Error("message should not be empty")
	}
}
