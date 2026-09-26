package handler

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/blivedm-go/message"
	"google.golang.org/protobuf/encoding/protowire"
)

func TestSendGiftV2BlindBoxProfit(t *testing.T) {
	// The field numbers match the live SEND_GIFT_V2 gift and blind-gift schema.
	blind := protowire.AppendString(nil, "心动盲盒")
	blind = append(protowire.AppendTag(nil, 3, protowire.BytesType), blind...)
	blind = protowire.AppendTag(blind, 6, protowire.VarintType)
	blind = protowire.AppendVarint(blind, 1500) // box unit cost

	gift := protowire.AppendTag(nil, 2, protowire.BytesType)
	gift = protowire.AppendString(gift, "开出的礼物")
	gift = protowire.AppendTag(gift, 3, protowire.VarintType)
	gift = protowire.AppendVarint(gift, 2)
	gift = protowire.AppendTag(gift, 5, protowire.VarintType)
	gift = protowire.AppendVarint(gift, 2000) // revealed unit value
	gift = protowire.AppendTag(gift, 6, protowire.VarintType)
	gift = protowire.AppendVarint(gift, 1500) // discounted unit price
	gift = protowire.AppendTag(gift, 7, protowire.VarintType)
	gift = protowire.AppendVarint(gift, 3000) // paid total

	payload := protowire.AppendTag(nil, 1, protowire.VarintType)
	payload = protowire.AppendVarint(payload, 123)
	payload = protowire.AppendTag(payload, 2, protowire.BytesType)
	payload = protowire.AppendString(payload, "送礼用户")
	payload = protowire.AppendTag(payload, 9, protowire.BytesType)
	payload = protowire.AppendBytes(payload, blind)
	payload = protowire.AppendTag(payload, 10, protowire.BytesType)
	payload = protowire.AppendBytes(payload, gift)

	envelope, err := json.Marshal(map[string]any{
		"data": map[string]string{"pb": base64.StdEncoding.EncodeToString(payload)},
	})
	if err != nil {
		t.Fatal(err)
	}
	broadcast, err := message.DecodeSendGiftV2(string(envelope))
	if err != nil {
		t.Fatal(err)
	}
	if len(broadcast.Gifts) != 1 {
		t.Fatalf("gift count = %d, want 1", len(broadcast.Gifts))
	}
	send := sendGiftV2ToLegacy(broadcast, broadcast.Gifts[0])
	if send.Data.Price != 2000 || send.Data.TotalCoin != 3000 || send.Data.BlindGift.OriginalGiftPrice != 1500 {
		t.Fatalf("prices: revealed=%d paid=%d box=%d", send.Data.Price, send.Data.TotalCoin, send.Data.BlindGift.OriginalGiftPrice)
	}
	profit := (send.Data.Price - send.Data.BlindGift.OriginalGiftPrice) * send.Data.Num
	if profit != 1000 {
		t.Fatalf("blind-box profit = %d, want 1000", profit)
	}
}
