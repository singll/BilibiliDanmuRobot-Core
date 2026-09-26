package message

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
)

// SendGiftV2 is the part of Bilibili's SendGiftBroadcast protobuf that the
// robot needs for gift thanks and blind-box statistics.
type SendGiftV2 struct {
	UID        uint64
	Uname      string
	Face       string
	GuardLevel uint64
	Medal      SendGiftV2Medal
	BlindGift  SendGiftV2BlindGift
	Gifts      []SendGiftV2Item
}

type SendGiftV2Medal struct {
	TargetID     uint64
	AnchorRoomID uint64
	Level        uint64
	Name         string
}

type SendGiftV2BlindGift struct {
	OriginalGiftName  string
	OriginalGiftPrice uint64
}

type SendGiftV2Item struct {
	GiftID        uint64
	GiftName      string
	Num           uint64
	GiftType      uint64
	Price         uint64
	DiscountPrice uint64
	TotalCoin     uint64
	CoinType      string
	TID           string
	Timestamp     uint64
	Rnd           string
	Action        string
}

// DecodeSendGiftV2 decodes the data.pb base64 envelope carried by SEND_GIFT_V2.
func DecodeSendGiftV2(raw string) (*SendGiftV2, error) {
	var envelope struct {
		Data struct {
			PB string `json:"pb"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return nil, fmt.Errorf("decode SEND_GIFT_V2 envelope: %w", err)
	}
	if envelope.Data.PB == "" {
		return nil, fmt.Errorf("SEND_GIFT_V2 envelope has no data.pb")
	}
	payload, err := base64.StdEncoding.DecodeString(envelope.Data.PB)
	if err != nil {
		return nil, fmt.Errorf("decode SEND_GIFT_V2 data.pb: %w", err)
	}
	result := new(SendGiftV2)
	if err := decodeSendGiftBroadcast(payload, result); err != nil {
		return nil, fmt.Errorf("decode SEND_GIFT_V2 protobuf: %w", err)
	}
	if len(result.Gifts) == 0 {
		return nil, fmt.Errorf("SEND_GIFT_V2 protobuf has no gift_list entries")
	}
	return result, nil
}

func decodeSendGiftBroadcast(b []byte, out *SendGiftV2) error {
	return walkProto(b, func(num protowire.Number, typ protowire.Type, value []byte) error {
		if err := requireWireType(num, typ, map[protowire.Number]protowire.Type{
			1: protowire.VarintType, 2: protowire.BytesType, 3: protowire.BytesType,
			5: protowire.VarintType, 8: protowire.BytesType, 9: protowire.BytesType,
			10: protowire.BytesType,
		}); err != nil {
			return err
		}
		switch num {
		case 1:
			out.UID = asVarint(value)
		case 2:
			out.Uname = string(value)
		case 3:
			out.Face = string(value)
		case 5:
			out.GuardLevel = asVarint(value)
		case 8:
			return decodeSendGiftMedal(value, &out.Medal)
		case 9:
			return decodeSendGiftBlindGift(value, &out.BlindGift)
		case 10:
			gift := SendGiftV2Item{}
			if err := decodeSendGiftItem(value, &gift); err != nil {
				return err
			}
			out.Gifts = append(out.Gifts, gift)
		}
		return nil
	})
}

func decodeSendGiftMedal(b []byte, out *SendGiftV2Medal) error {
	return walkProto(b, func(num protowire.Number, typ protowire.Type, value []byte) error {
		if err := requireWireType(num, typ, map[protowire.Number]protowire.Type{
			1: protowire.VarintType, 4: protowire.VarintType,
			5: protowire.VarintType, 6: protowire.BytesType,
		}); err != nil {
			return err
		}
		switch num {
		case 1:
			out.TargetID = asVarint(value)
		case 4:
			out.AnchorRoomID = asVarint(value)
		case 5:
			out.Level = asVarint(value)
		case 6:
			out.Name = string(value)
		}
		return nil
	})
}

func decodeSendGiftBlindGift(b []byte, out *SendGiftV2BlindGift) error {
	return walkProto(b, func(num protowire.Number, typ protowire.Type, value []byte) error {
		if err := requireWireType(num, typ, map[protowire.Number]protowire.Type{
			3: protowire.BytesType, 6: protowire.VarintType,
		}); err != nil {
			return err
		}
		switch num {
		case 3:
			out.OriginalGiftName = string(value)
		case 6:
			out.OriginalGiftPrice = asVarint(value)
		}
		return nil
	})
}

func decodeSendGiftItem(b []byte, out *SendGiftV2Item) error {
	return walkProto(b, func(num protowire.Number, typ protowire.Type, value []byte) error {
		if err := requireWireType(num, typ, map[protowire.Number]protowire.Type{
			1: protowire.VarintType, 2: protowire.BytesType, 3: protowire.VarintType,
			4: protowire.VarintType, 5: protowire.VarintType, 6: protowire.VarintType,
			7: protowire.VarintType,
			8: protowire.BytesType, 9: protowire.BytesType, 10: protowire.VarintType,
			12: protowire.BytesType, 18: protowire.BytesType,
		}); err != nil {
			return err
		}
		switch num {
		case 1:
			out.GiftID = asVarint(value)
		case 2:
			out.GiftName = string(value)
		case 3:
			out.Num = asVarint(value)
		case 4:
			out.GiftType = asVarint(value)
		case 5:
			out.Price = asVarint(value)
		case 6:
			out.TotalCoin = asVarint(value)
		case 7:
			out.DiscountPrice = asVarint(value)
		case 8:
			out.CoinType = string(value)
		case 9:
			out.TID = string(value)
		case 10:
			out.Timestamp = asVarint(value)
		case 12:
			out.Rnd = string(value)
		case 18:
			out.Action = string(value)
		}
		return nil
	})
}

func requireWireType(num protowire.Number, actual protowire.Type, known map[protowire.Number]protowire.Type) error {
	if expected, ok := known[num]; ok && expected != actual {
		return fmt.Errorf("field %d has wire type %d, want %d", num, actual, expected)
	}
	return nil
}

// walkProto validates each field and skips fields added by future Bilibili
// protobuf versions. The callback receives the decoded value without its tag.
func walkProto(b []byte, visit func(protowire.Number, protowire.Type, []byte) error) error {
	for len(b) > 0 {
		num, typ, tagLen := protowire.ConsumeTag(b)
		if tagLen < 0 {
			return protowire.ParseError(tagLen)
		}
		b = b[tagLen:]
		var value []byte
		switch typ {
		case protowire.VarintType:
			_, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			value = b[:n]
			b = b[n:]
		case protowire.BytesType:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			value = v
			b = b[n:]
		default:
			n := protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				return protowire.ParseError(n)
			}
			b = b[n:]
			continue
		}
		if err := visit(num, typ, value); err != nil {
			return err
		}
	}
	return nil
}

func asVarint(b []byte) uint64 {
	v, _ := protowire.ConsumeVarint(b)
	return v
}
