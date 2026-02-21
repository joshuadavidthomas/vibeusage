package antigravity

import (
	"encoding/base64"

	"google.golang.org/protobuf/encoding/protowire"
)

// SubscriptionInfo holds the subscription tier parsed from the protobuf
// blob in the Antigravity vscdb auth status.
type SubscriptionInfo struct {
	TierID   string // e.g. "g1-pro-tier"
	TierName string // e.g. "Google AI Pro"
}

// parseSubscriptionFromProto extracts subscription info from the
// userStatusProtoBinaryBase64 field in the vscdb auth status.
//
// The protobuf structure (reverse-engineered):
//
//	Top-level message:
//	  field 3  (string): user name
//	  field 7  (string): user email
//	  field 13 (message): PlanStatus (contains PlanInfo with plan name "Pro")
//	  field 36 (message): SubscriptionInfo
//	    field 1 (string): tier ID (e.g. "g1-pro-tier")
//	    field 2 (string): tier name (e.g. "Google AI Pro")
func parseSubscriptionFromProto(base64Value string) *SubscriptionInfo {
	if base64Value == "" {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(base64Value)
	if err != nil {
		return nil
	}

	// Walk top-level fields looking for field 36 (subscription info)
	subscriptionBytes := findField(data, 36)
	if subscriptionBytes == nil {
		return nil
	}

	info := &SubscriptionInfo{
		TierID:   string(findField(subscriptionBytes, 1)),
		TierName: string(findField(subscriptionBytes, 2)),
	}

	if info.TierID == "" && info.TierName == "" {
		return nil
	}
	return info
}

// findField walks a protobuf message and returns the raw bytes of the
// first length-delimited field matching the given field number.
func findField(b []byte, target protowire.Number) []byte {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil
		}
		b = b[n:]

		switch typ {
		case protowire.VarintType:
			_, n = protowire.ConsumeVarint(b)
		case protowire.Fixed32Type:
			_, n = protowire.ConsumeFixed32(b)
		case protowire.Fixed64Type:
			_, n = protowire.ConsumeFixed64(b)
		case protowire.BytesType:
			val, vn := protowire.ConsumeBytes(b)
			if vn < 0 {
				return nil
			}
			if num == target {
				return val
			}
			n = vn
		case protowire.StartGroupType:
			_, n = protowire.ConsumeGroup(num, b)
		default:
			return nil
		}

		if n < 0 {
			return nil
		}
		b = b[n:]
	}
	return nil
}
