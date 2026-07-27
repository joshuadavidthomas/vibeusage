package provider

import "testing"

func TestPrepareCredential(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "trims surrounding whitespace", input: "  secret\n", want: "secret"},
		{name: "rejects empty", input: " \t\n", wantErr: true},
		{name: "rejects embedded newline", input: "secret\nvalue", wantErr: true},
		{name: "rejects null byte", input: "secret\x00value", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PrepareCredential(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("PrepareCredential() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("PrepareCredential() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestManualKeyAuthFlowPrepareCookie(t *testing.T) {
	flow := ManualKeyAuthFlow{
		CookieNames: []string{"auth"},
		Validate:    ValidateNotEmpty,
	}
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "cookie value", input: "session-value==", want: "session-value=="},
		{name: "cookie assignment", input: "auth=session-value", wantErr: true},
		{name: "case-varied assignment", input: "AUTH=session-value", wantErr: true},
		{name: "spaced assignment", input: "auth =session-value", wantErr: true},
		{name: "cookie header", input: "Cookie: auth=session-value", wantErr: true},
		{name: "spaced cookie header", input: "Cookie : auth=session-value", wantErr: true},
		{name: "set-cookie header", input: "Set-Cookie: auth=session-value", wantErr: true},
		{name: "multiple cookies", input: "session-value; other=value", wantErr: true},
		{name: "quoted value", input: `"session-value"`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := flow.Prepare(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Prepare() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Prepare() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidatePrefix(t *testing.T) {
	validate := ValidatePrefix("sk-ant-sid01-")

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid key passes", "sk-ant-sid01-abc123", false},
		{"prefix only passes", "sk-ant-sid01-", false},
		{"wrong prefix fails", "some-random-key", true},
		{"empty fails", "", true},
		{"whitespace only fails", "   ", true},
		{"whitespace around valid key passes", "  sk-ant-sid01-abc  ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePrefix(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateNotEmpty(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"non-empty string passes", "hello", false},
		{"empty string fails", "", true},
		{"whitespace-only fails", "   ", true},
		{"tab-only fails", "\t", true},
		{"value with spaces passes", "  hello  ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNotEmpty(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNotEmpty(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
