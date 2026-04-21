package claude

import (
	"encoding/json"
	"testing"
)

func TestWebOrganization_UnmarshalWithUUID(t *testing.T) {
	raw := `{
		"uuid": "org-uuid-123",
		"name": "Test Org",
		"capabilities": ["chat", "billing"]
	}`

	var org WebOrganization
	if err := json.Unmarshal([]byte(raw), &org); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if org.UUID != "org-uuid-123" {
		t.Errorf("uuid = %q, want %q", org.UUID, "org-uuid-123")
	}
	if org.Name != "Test Org" {
		t.Errorf("name = %q, want %q", org.Name, "Test Org")
	}
	if len(org.Capabilities) != 2 {
		t.Fatalf("len(capabilities) = %d, want 2", len(org.Capabilities))
	}
	if org.Capabilities[0] != "chat" {
		t.Errorf("capabilities[0] = %q, want %q", org.Capabilities[0], "chat")
	}
	if org.Capabilities[1] != "billing" {
		t.Errorf("capabilities[1] = %q, want %q", org.Capabilities[1], "billing")
	}
}

func TestWebOrganization_UnmarshalWithID(t *testing.T) {
	raw := `{
		"id": "org-id-456",
		"capabilities": ["chat"]
	}`

	var org WebOrganization
	if err := json.Unmarshal([]byte(raw), &org); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if org.ID != "org-id-456" {
		t.Errorf("id = %q, want %q", org.ID, "org-id-456")
	}
}

func TestWebOrganization_UnmarshalBothUUIDAndID(t *testing.T) {
	raw := `{
		"uuid": "org-uuid-123",
		"id": "org-id-456",
		"capabilities": ["chat"]
	}`

	var org WebOrganization
	if err := json.Unmarshal([]byte(raw), &org); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if org.UUID != "org-uuid-123" {
		t.Errorf("uuid = %q, want %q", org.UUID, "org-uuid-123")
	}
	if org.ID != "org-id-456" {
		t.Errorf("id = %q, want %q", org.ID, "org-id-456")
	}
}

func TestWebOrganization_UnmarshalNoCaps(t *testing.T) {
	raw := `{
		"uuid": "org-uuid-123"
	}`

	var org WebOrganization
	if err := json.Unmarshal([]byte(raw), &org); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if org.UUID != "org-uuid-123" {
		t.Errorf("uuid = %q, want %q", org.UUID, "org-uuid-123")
	}
	if org.Capabilities != nil {
		t.Errorf("capabilities = %v, want nil", org.Capabilities)
	}
}

func TestWebOrganization_HasCapability(t *testing.T) {
	tests := []struct {
		name       string
		org        WebOrganization
		capability string
		want       bool
	}{
		{
			name:       "has chat capability",
			org:        WebOrganization{Capabilities: []string{"chat", "billing"}},
			capability: "chat",
			want:       true,
		},
		{
			name:       "missing capability",
			org:        WebOrganization{Capabilities: []string{"billing"}},
			capability: "chat",
			want:       false,
		},
		{
			name:       "nil capabilities",
			org:        WebOrganization{},
			capability: "chat",
			want:       false,
		},
		{
			name:       "empty capabilities",
			org:        WebOrganization{Capabilities: []string{}},
			capability: "chat",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.org.HasCapability(tt.capability)
			if got != tt.want {
				t.Errorf("HasCapability(%q) = %v, want %v", tt.capability, got, tt.want)
			}
		})
	}
}

func TestWebOrganization_OrgID(t *testing.T) {
	tests := []struct {
		name string
		org  WebOrganization
		want string
	}{
		{
			name: "uuid takes precedence",
			org:  WebOrganization{UUID: "org-uuid", ID: "org-id"},
			want: "org-uuid",
		},
		{
			name: "fallback to id",
			org:  WebOrganization{ID: "org-id"},
			want: "org-id",
		},
		{
			name: "both empty",
			org:  WebOrganization{},
			want: "",
		},
		{
			name: "uuid only",
			org:  WebOrganization{UUID: "org-uuid"},
			want: "org-uuid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.org.OrgID()
			if got != tt.want {
				t.Errorf("OrgID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWebOrganization_UnmarshalOrganizationsList(t *testing.T) {
	raw := `[
		{"uuid": "org-1", "name": "Personal", "capabilities": ["chat"]},
		{"uuid": "org-2", "name": "Work", "capabilities": ["chat", "billing"]},
		{"id": "org-3", "name": "Legacy", "capabilities": ["billing"]}
	]`

	var orgs []WebOrganization
	if err := json.Unmarshal([]byte(raw), &orgs); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(orgs) != 3 {
		t.Fatalf("len(orgs) = %d, want 3", len(orgs))
	}

	if orgs[0].UUID != "org-1" {
		t.Errorf("orgs[0].uuid = %q, want %q", orgs[0].UUID, "org-1")
	}
	if orgs[0].Name != "Personal" {
		t.Errorf("orgs[0].name = %q, want %q", orgs[0].Name, "Personal")
	}
	if !orgs[0].HasCapability("chat") {
		t.Error("orgs[0] should have chat capability")
	}

	if orgs[2].ID != "org-3" {
		t.Errorf("orgs[2].id = %q, want %q", orgs[2].ID, "org-3")
	}
	if orgs[2].HasCapability("chat") {
		t.Error("orgs[2] should not have chat capability")
	}
}

func TestWebSessionCredentials_Unmarshal(t *testing.T) {
	raw := `{"session_key": "sk-ant-sid01-test-key"}`

	var creds WebSessionCredentials
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if creds.SessionKey != "sk-ant-sid01-test-key" {
		t.Errorf("session_key = %q, want %q", creds.SessionKey, "sk-ant-sid01-test-key")
	}
}

func TestWebSessionCredentials_UnmarshalEmpty(t *testing.T) {
	raw := `{}`

	var creds WebSessionCredentials
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if creds.SessionKey != "" {
		t.Errorf("session_key = %q, want empty", creds.SessionKey)
	}
}
