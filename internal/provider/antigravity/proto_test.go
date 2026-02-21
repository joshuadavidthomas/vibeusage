package antigravity

import (
	"encoding/base64"
	"testing"
)

func TestParseSubscriptionFromProto_ProTier(t *testing.T) {
	// Build a minimal protobuf with field 36 containing subscription info:
	// field 36 (message): { field 1: "g1-pro-tier", field 2: "Google AI Pro" }
	inner := buildLengthDelimited(1, []byte("g1-pro-tier"))
	inner = append(inner, buildLengthDelimited(2, []byte("Google AI Pro"))...)
	proto := buildLengthDelimited(36, inner)

	b64 := base64.StdEncoding.EncodeToString(proto)
	info := parseSubscriptionFromProto(b64)

	if info == nil {
		t.Fatal("expected non-nil subscription info")
	}
	if info.TierID != "g1-pro-tier" {
		t.Errorf("tierID = %q, want %q", info.TierID, "g1-pro-tier")
	}
	if info.TierName != "Google AI Pro" {
		t.Errorf("tierName = %q, want %q", info.TierName, "Google AI Pro")
	}
}

func TestParseSubscriptionFromProto_NoField36(t *testing.T) {
	// Protobuf with only field 3 (name) — no subscription
	proto := buildLengthDelimited(3, []byte("Test User"))
	b64 := base64.StdEncoding.EncodeToString(proto)

	info := parseSubscriptionFromProto(b64)
	if info != nil {
		t.Errorf("expected nil, got %+v", info)
	}
}

func TestParseSubscriptionFromProto_Empty(t *testing.T) {
	info := parseSubscriptionFromProto("")
	if info != nil {
		t.Errorf("expected nil for empty input, got %+v", info)
	}
}

func TestParseSubscriptionFromProto_InvalidBase64(t *testing.T) {
	info := parseSubscriptionFromProto("not-valid-base64!!!")
	if info != nil {
		t.Errorf("expected nil for invalid base64, got %+v", info)
	}
}

func TestReadVarint(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    uint64
		wantOff int
	}{
		{"single byte", []byte{0x08}, 8, 1},
		{"multi byte", []byte{0x80, 0x01}, 128, 2},
		{"zero", []byte{0x00}, 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, off := readVarint(tt.data, 0)
			if val != tt.want {
				t.Errorf("val = %d, want %d", val, tt.want)
			}
			if off != tt.wantOff {
				t.Errorf("offset = %d, want %d", off, tt.wantOff)
			}
		})
	}
}

// buildLengthDelimited builds a protobuf field with wire type 2 (length-delimited).
func buildLengthDelimited(fieldNum int, value []byte) []byte {
	tag := uint64((fieldNum << 3) | 2)
	var result []byte
	result = appendVarint(result, tag)
	result = appendVarint(result, uint64(len(value)))
	result = append(result, value...)
	return result
}

func appendVarint(buf []byte, val uint64) []byte {
	for val >= 0x80 {
		buf = append(buf, byte(val)|0x80)
		val >>= 7
	}
	buf = append(buf, byte(val))
	return buf
}
