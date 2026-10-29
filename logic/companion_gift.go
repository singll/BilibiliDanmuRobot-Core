package logic

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/companion"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/entity"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/svc"
)

type giftBatch struct {
	uid         int
	name        string
	gifts       map[string]int
	cost        int
	first, last time.Time
}
type giftBatches map[int]*giftBatch

func (b giftBatches) add(g *entity.SendGiftText, now time.Time) {
	item := b[g.Data.UID]
	if item == nil {
		item = &giftBatch{uid: g.Data.UID, name: g.Data.Uname, gifts: make(map[string]int), first: now}
		b[g.Data.UID] = item
	}
	name := g.Data.GiftName
	if g.Data.BlindGift.OriginalGiftName != "" {
		name += "（来自" + g.Data.BlindGift.OriginalGiftName + "）"
	}
	item.gifts[name] += g.Data.Num
	item.last = now
	cost := g.Data.TotalCoin
	if cost == 0 {
		cost = g.Data.Price * g.Data.Num
	}
	item.cost += cost
}
func (b giftBatches) due(now time.Time, quiet, maxWait time.Duration) []*giftBatch {
	var out []*giftBatch
	for id, g := range b {
		if now.Sub(g.last) >= quiet || now.Sub(g.first) >= maxWait {
			out = append(out, g)
			delete(b, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].first.Before(out[j].first) })
	return out
}
func (g *giftBatch) summary() string {
	names := make([]string, 0, len(g.gifts))
	for n := range g.gifts {
		names = append(names, n)
	}
	sort.Strings(names)
	var parts []string
	for _, n := range names {
		parts = append(parts, fmt.Sprintf("%d个%s", g.gifts[n], n))
	}
	return strings.Join(parts, "、")
}

func companionGifts(ctx context.Context, svcCtx *svc.ServiceContext) {
	batches := make(giftBatches)
	var epoch uint64
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	quiet := time.Duration(svcCtx.Config.ThanksGiftTimeout) * time.Second
	if quiet <= 0 {
		quiet = 8 * time.Second
	}
	maxWait := time.Duration(svcCtx.Config.Companion.GiftMaxWaitSeconds) * time.Second
	if maxWait <= 0 {
		maxWait = 20 * time.Second
	}
	for {
		select {
		case <-ctx.Done():
			return
		case g := <-thanksGiver.giftChan:
			now := time.Now()
			current, live := svcCtx.Companion.Session(now)
			if current != epoch || !live {
				batches = make(giftBatches)
				epoch = current
			}
			if !live || (g.Data.Timestamp > 0 && now.Unix()-int64(g.Data.Timestamp) > 30) {
				continue
			}
			batches.add(g, now)
		case now := <-ticker.C:
			current, live := svcCtx.Companion.Session(now)
			if current != epoch || !live {
				batches = make(giftBatches)
				epoch = current
				continue
			}
			for _, b := range batches.due(now, quiet, maxWait) {
				if b.cost < svcCtx.Config.ThanksMinCost {
					continue
				}
				var replies []*entity.DanmuMsgTextReplyInfo
				if svcCtx.Config.ThanksGiftUseAt {
					replies = []*entity.DanmuMsgTextReplyInfo{{ReplyUid: strconv.Itoa(b.uid), ReplyName: b.name}}
				}
				GiftPersona(svcCtx, "观众"+b.name+"赠送了"+b.summary(), "谢谢"+b.name+"的礼物。", replies...)
			}
		}
	}
}

func GiftPersona(svcCtx *svc.ServiceContext, facts, fallback string, reply ...*entity.DanmuMsgTextReplyInfo) {
	name := ""
	if len(reply) > 0 && reply[0] != nil {
		name = reply[0].ReplyName
	}
	svcCtx.Companion.Submit(companion.Event{Scene: "gift", Facts: facts, Name: name, Fallback: fallback}, 60*time.Second, func(s string, valid func() bool) { PushPersona(s, valid, reply...) })
}
