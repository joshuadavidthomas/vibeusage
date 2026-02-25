package catalog

import (
	"testing"
)

func TestParseMultipliersYAML(t *testing.T) {
	yaml := `# Comment
- name: Claude Sonnet 4.6
  multiplier_paid: 1
  multiplier_free: Not applicable

- name: GPT-4o
  multiplier_paid: 0
  multiplier_free: 1

- name: Claude Opus 4.6
  multiplier_paid: 3
  multiplier_free: Not applicable

- name: Claude Haiku 4.5
  multiplier_paid: 0.33
  multiplier_free: 1

- name: Goldeneye
  multiplier_paid: Not applicable
  multiplier_free: 1
`
	entries := parseMultipliersYAML(yaml)

	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}

	// Claude Sonnet 4.6: paid=1, free=nil
	if entries[0].Name != "Claude Sonnet 4.6" {
		t.Errorf("entry 0 name = %q", entries[0].Name)
	}
	if entries[0].Paid == nil || *entries[0].Paid != 1 {
		t.Errorf("entry 0 paid = %v, want 1", entries[0].Paid)
	}
	if entries[0].Free != nil {
		t.Errorf("entry 0 free = %v, want nil (Not applicable)", entries[0].Free)
	}

	// GPT-4o: paid=0, free=1
	if entries[1].Paid == nil || *entries[1].Paid != 0 {
		t.Errorf("GPT-4o paid = %v, want 0", entries[1].Paid)
	}
	if entries[1].Free == nil || *entries[1].Free != 1 {
		t.Errorf("GPT-4o free = %v, want 1", entries[1].Free)
	}

	// Claude Opus 4.6: paid=3
	if entries[2].Paid == nil || *entries[2].Paid != 3 {
		t.Errorf("Opus paid = %v, want 3", entries[2].Paid)
	}

	// Claude Haiku 4.5: paid=0.33
	if entries[3].Paid == nil || *entries[3].Paid != 0.33 {
		t.Errorf("Haiku paid = %v, want 0.33", entries[3].Paid)
	}

	// Goldeneye: paid=nil (Not applicable)
	if entries[4].Paid != nil {
		t.Errorf("Goldeneye paid = %v, want nil", entries[4].Paid)
	}
}
