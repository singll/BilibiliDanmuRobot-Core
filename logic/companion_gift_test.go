package logic

import (
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/entity"
	"testing"
	"time"
)

func TestGiftBatchesMergeAndMaxWait(t *testing.T) {
	now := time.Now()
	b := make(giftBatches)
	gift := &entity.SendGiftText{}
	gift.Data.UID = 7
	gift.Data.Uname = "小鱼"
	gift.Data.GiftName = "小花花"
	gift.Data.Num = 3
	gift.Data.Price = 100
	b.add(gift, now)
	b.add(gift, now.Add(7*time.Second))
	b.add(gift, now.Add(14*time.Second))
	if len(b.due(now.Add(19*time.Second), 8*time.Second, 20*time.Second)) != 0 {
		t.Fatal("premature flush")
	}
	due := b.due(now.Add(20*time.Second), 8*time.Second, 20*time.Second)
	if len(due) != 1 || due[0].gifts["小花花"] != 9 || due[0].cost != 900 || due[0].summary() != "9个小花花" {
		t.Fatalf("bad aggregate: %+v", due)
	}
	if len(b) != 0 {
		t.Fatal("not consumed")
	}
}
func TestGiftBatchesSeparateUsers(t *testing.T) {
	now := time.Now()
	b := make(giftBatches)
	for _, id := range []int{1, 2} {
		g := &entity.SendGiftText{}
		g.Data.UID = id
		g.Data.Uname = "同名观众"
		g.Data.GiftName = "花"
		g.Data.Num = 1
		b.add(g, now)
	}
	if len(b.due(now.Add(8*time.Second), 8*time.Second, 20*time.Second)) != 2 {
		t.Fatal("same names merged different viewers")
	}
}
