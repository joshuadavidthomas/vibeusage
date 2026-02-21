package antigravity

import (
	"encoding/base64"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestParseSubscriptionFromProto_ProTier(t *testing.T) {
	inner := protoBytes(1, []byte("g1-pro-tier"))
	inner = append(inner, protoBytes(2, []byte("Google AI Pro"))...)
	proto := protoBytes(36, inner)

	info := parseSubscriptionFromProto(base64.StdEncoding.EncodeToString(proto))

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

func TestParseSubscriptionFromProto_WithOtherFields(t *testing.T) {
	// Simulate a real proto with other fields before field 36
	var proto []byte
	proto = append(proto, protoBytes(3, []byte("Joshua Thomas"))...)
	proto = append(proto, protoBytes(7, []byte("user@example.com"))...)

	inner := protoBytes(1, []byte("g1-ultra-tier"))
	inner = append(inner, protoBytes(2, []byte("Google AI Ultra"))...)
	proto = append(proto, protoBytes(36, inner)...)

	info := parseSubscriptionFromProto(base64.StdEncoding.EncodeToString(proto))

	if info == nil {
		t.Fatal("expected non-nil subscription info")
	}
	if info.TierName != "Google AI Ultra" {
		t.Errorf("tierName = %q, want %q", info.TierName, "Google AI Ultra")
	}
}

func TestParseSubscriptionFromProto_NoField36(t *testing.T) {
	proto := protoBytes(3, []byte("Test User"))

	info := parseSubscriptionFromProto(base64.StdEncoding.EncodeToString(proto))
	if info != nil {
		t.Errorf("expected nil, got %+v", info)
	}
}

func TestParseSubscriptionFromProto_Empty(t *testing.T) {
	if info := parseSubscriptionFromProto(""); info != nil {
		t.Errorf("expected nil for empty input, got %+v", info)
	}
}

func TestParseSubscriptionFromProto_InvalidBase64(t *testing.T) {
	if info := parseSubscriptionFromProto("not-valid-base64!!!"); info != nil {
		t.Errorf("expected nil for invalid base64, got %+v", info)
	}
}

// protoBytes encodes a length-delimited protobuf field.
func protoBytes(num protowire.Number, val []byte) []byte {
	b := protowire.AppendTag(nil, num, protowire.BytesType)
	b = protowire.AppendBytes(b, val)
	return b
}
