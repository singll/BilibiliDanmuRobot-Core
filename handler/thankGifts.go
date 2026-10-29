package handler

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/blivedm-go/message"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/entity"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/logic"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/logic/danmu"
	"github.com/zeromicro/go-zero/core/logx"
)

// 礼物感谢
func (w *wsHandler) thankGifts() {
	w.client.RegisterCustomEventHandler("SEND_GIFT", func(s string) {
		send := &entity.SendGiftText{}
		if err := json.Unmarshal([]byte(s), send); err != nil {
			logx.Errorf("decode SEND_GIFT: %v", err)
			return
		}
		w.svc.Companion.Activity(time.Now())
		if w.svc.Config.ThanksGift {
			logic.PushToGiftChan(send)
		}
		danmu.SaveBlindBoxStat(send, w.svc)
	})
	w.client.RegisterCustomEventHandler("SEND_GIFT_V2", func(s string) {
		broadcast, err := message.DecodeSendGiftV2(s)
		if err != nil {
			logx.Errorf("decode SEND_GIFT_V2: %v", err)
			return
		}
		logx.Infof("SEND_GIFT_V2 decoded: gifts=%d thanks_enabled=%t", len(broadcast.Gifts), w.svc.Config.ThanksGift)
		for _, gift := range broadcast.Gifts {
			send := sendGiftV2ToLegacy(broadcast, gift)
			w.svc.Companion.Activity(time.Now())
			if w.svc.Config.ThanksGift {
				logic.PushToGiftChan(send)
			}
			danmu.SaveBlindBoxStat(send, w.svc)
		}
	})
	w.client.RegisterCustomEventHandler("GUARD_BUY", func(s string) {
		if w.svc.Config.ThanksGift {
			send := &entity.GuardBuyText{}
			_ = json.Unmarshal([]byte(s), send)
			w.svc.Companion.Activity(time.Now())
			if w.svc.Config.Companion.Enabled && w.svc.Config.Companion.Gift {
				var replies []*entity.DanmuMsgTextReplyInfo
				if w.svc.Config.ThanksGiftUseAt {
					replies = []*entity.DanmuMsgTextReplyInfo{{ReplyUid: strconv.Itoa(send.Data.Uid), ReplyName: send.Data.Username}}
				}
				logic.GiftPersona(w.svc, "观众"+send.Data.Username+"开通/续费了"+send.Data.GiftName, "谢谢"+send.Data.Username+"的支持。", replies...)
				return
			}
			if w.svc.Config.ThanksGiftUseAt {
				logic.PushToGuardChan(send, &entity.DanmuMsgTextReplyInfo{
					ReplyUid:  strconv.Itoa(send.Data.Uid),
					ReplyName: send.Data.Username,
				})
			} else {
				logic.PushToGuardChan(send)
			}
		}
	})

	w.client.RegisterCustomEventHandler("COMMON_NOTICE_DANMAKU", func(s string) {
		if w.svc.Config.ThanksGift {
			data := &entity.CommonNoticeDanmaku{}
			_ = json.Unmarshal([]byte(s), data)
			if len(data.Data.ContentSegments) == 5 &&
				data.Data.ContentSegments[1].Text == "投喂" &&
				data.Data.ContentSegments[2].Text == "大航海盲盒" {

				msg := fmt.Sprintf("感谢 %s 的 %s", data.Data.ContentSegments[0].Text, data.Data.ContentSegments[4].Text)
				w.svc.Companion.Activity(time.Now())
				if w.svc.Config.Companion.Enabled && w.svc.Config.Companion.Gift {
					logic.GiftPersona(w.svc, msg, msg)
				} else {
					logic.PushToBulletSender(msg)
				}
			} else if len(data.Data.ContentSegments) == 6 &&
				data.Data.ContentSegments[2].Text == "投喂" &&
				data.Data.ContentSegments[3].Text == "大航海盲盒" {

				msg := fmt.Sprintf("感谢 %s 的 %s", data.Data.ContentSegments[1].Text, data.Data.ContentSegments[5].Text)
				w.svc.Companion.Activity(time.Now())
				if w.svc.Config.Companion.Enabled && w.svc.Config.Companion.Gift {
					logic.GiftPersona(w.svc, msg, msg)
				} else {
					logic.PushToBulletSender(msg)
				}
			}
		}
	})
}

func sendGiftV2ToLegacy(broadcast *message.SendGiftV2, gift message.SendGiftV2Item) *entity.SendGiftText {
	send := &entity.SendGiftText{Cmd: "SEND_GIFT_V2"}
	send.Data.Action = gift.Action
	send.Data.CoinType = gift.CoinType
	send.Data.Face = broadcast.Face
	send.Data.Giftid = int(gift.GiftID)
	send.Data.GiftName = gift.GiftName
	send.Data.Gifttype = int(gift.GiftType)
	send.Data.GuardLevel = int(broadcast.GuardLevel)
	send.Data.Num = int(gift.Num)
	send.Data.OriginalGiftName = broadcast.BlindGift.OriginalGiftName
	// Price is the revealed item's unit value. TotalCoin is the whole send's
	// paid amount, so it must not be used to calculate blind-box profit.
	unitPrice := gift.Price
	if unitPrice == 0 && broadcast.BlindGift.OriginalGiftName == "" {
		unitPrice = gift.DiscountPrice
	}
	send.Data.Price = int(unitPrice)
	send.Data.Rnd = gift.Rnd
	send.Data.Tid = gift.TID
	send.Data.Timestamp = int(gift.Timestamp)
	send.Data.TotalCoin = int(gift.TotalCoin)
	if send.Data.TotalCoin == 0 {
		paidUnitPrice := gift.DiscountPrice
		if broadcast.BlindGift.OriginalGiftName != "" {
			paidUnitPrice = broadcast.BlindGift.OriginalGiftPrice
		} else if paidUnitPrice == 0 {
			paidUnitPrice = unitPrice
		}
		send.Data.TotalCoin = int(paidUnitPrice * gift.Num)
	}
	send.Data.UID = int(broadcast.UID)
	send.Data.Uname = broadcast.Uname
	send.Data.BlindGift.OriginalGiftName = broadcast.BlindGift.OriginalGiftName
	send.Data.BlindGift.OriginalGiftPrice = int(broadcast.BlindGift.OriginalGiftPrice)
	send.Data.MedalInfo.TargetID = int(broadcast.Medal.TargetID)
	send.Data.MedalInfo.AnchorRoomid = int(broadcast.Medal.AnchorRoomID)
	send.Data.MedalInfo.MedalLevel = int(broadcast.Medal.Level)
	send.Data.MedalInfo.MedalName = broadcast.Medal.Name
	return send
}
