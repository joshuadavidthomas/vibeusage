package antigravity

import "encoding/base64"

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
	var subscriptionBytes []byte
	offset := 0
	for offset < len(data) {
		tag, newOffset := readVarint(data, offset)
		if newOffset == offset {
			break
		}
		offset = newOffset

		fieldNum := tag >> 3
		wireType := tag & 0x7

		switch wireType {
		case 0: // varint
			_, offset = readVarint(data, offset)
		case 2: // length-delimited
			length, newOff := readVarint(data, offset)
			offset = newOff
			if offset+int(length) > len(data) {
				return nil
			}
			if fieldNum == 36 {
				subscriptionBytes = data[offset : offset+int(length)]
			}
			offset += int(length)
		default:
			return nil // unknown wire type, bail
		}
	}

	if subscriptionBytes == nil {
		return nil
	}

	// Parse the subscription submessage: field 1 = tier ID, field 2 = tier name
	info := &SubscriptionInfo{}
	offset = 0
	for offset < len(subscriptionBytes) {
		tag, newOffset := readVarint(subscriptionBytes, offset)
		if newOffset == offset {
			break
		}
		offset = newOffset

		fieldNum := tag >> 3
		wireType := tag & 0x7

		switch wireType {
		case 0: // varint
			_, offset = readVarint(subscriptionBytes, offset)
		case 2: // length-delimited
			length, newOff := readVarint(subscriptionBytes, offset)
			offset = newOff
			if offset+int(length) > len(subscriptionBytes) {
				return info
			}
			val := subscriptionBytes[offset : offset+int(length)]
			offset += int(length)

			switch fieldNum {
			case 1:
				info.TierID = string(val)
			case 2:
				info.TierName = string(val)
			}
		default:
			return info
		}
	}

	if info.TierID == "" && info.TierName == "" {
		return nil
	}
	return info
}

// readVarint reads a protobuf varint from data at the given offset.
// Returns the value and the new offset. If the data is malformed,
// returns (0, offset) unchanged.
func readVarint(data []byte, offset int) (uint64, int) {
	var val uint64
	var shift uint
	for offset < len(data) {
		b := data[offset]
		offset++
		val |= uint64(b&0x7f) << shift
		shift += 7
		if b&0x80 == 0 {
			return val, offset
		}
	}
	return 0, offset
}
