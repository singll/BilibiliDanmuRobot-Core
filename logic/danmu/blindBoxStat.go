package danmu

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/entity"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/logic"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/model"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/svc"
	_ "github.com/glebarez/go-sqlite"
	"github.com/zeromicro/go-zero/core/logx"
)

var errInfo string = "盲盒统计服务异常"

func SaveBlindBoxStat(g *entity.SendGiftText, svcCtx *svc.ServiceContext) {
	logx.Info(g.Data.BlindGift.OriginalGiftName)
	if g.Data.BlindGift.OriginalGiftName == "" {
		return
	}
	now := time.Now().In(blindBoxLocation)
	err := svcCtx.BlindBoxStatModel.Insert(context.Background(), nil, &model.BlindBoxStatBase{
		Uid:               int64(g.Data.UID),
		BlindBoxName:      g.Data.BlindGift.OriginalGiftName,
		Price:             int32(g.Data.Price),
		OriginalGiftPrice: int32(g.Data.BlindGift.OriginalGiftPrice),
		Cnt:               int32(g.Data.Num),
		Year:              int16(now.Year()),
		Month:             int16(now.Month()),
		Day:               int16(now.Day()),
	})
	if err != nil {
		logx.Alert("保存盲盒数据出错!!! " + err.Error())
	} else {
		logx.Info("盲盒数据保存成功!!! ")
	}
}

func DoBlindBoxStat(msg, uid, username string, svcCtx *svc.ServiceContext, reply ...*entity.DanmuMsgTextReplyInfo) {
	if !svcCtx.Config.BlindBoxStat {
		return
	}

	response, err := queryBlindBoxStat(context.Background(), msg, uid, svcCtx, time.Now())
	if err != nil {
		logic.PushToBulletSender(errInfo, reply...)
		logx.Alert("盲盒统计出错了!" + err.Error())
		return
	}
	if response != "" {
		logic.PushToBulletSender(response, reply...)
	}
}

// queryBlindBoxStat does not enqueue or send messages, so query behavior can be
// verified without contacting Bilibili.
func queryBlindBoxStat(ctx context.Context, msg, uid string, svcCtx *svc.ServiceContext, now time.Time) (string, error) {
	period, matched, err := parseBlindBoxPeriod(msg, now)
	if !matched {
		return "", nil
	}
	if err != nil {
		return err.Error(), nil
	}
	id, err := strconv.ParseInt(uid, 10, 64)
	if err != nil || id <= 0 {
		return "", fmt.Errorf("invalid blind-box query uid %q", uid)
	}
	// Only the broadcaster may query the whole room. Everyone else sees their
	// own records, including for all-history queries.
	var filterUID *int64
	if svcCtx.UserID != id {
		filterUID = &id
	}
	ret, err := svcCtx.BlindBoxStatModel.GetTotalInRange(ctx, filterUID, period.start, period.end)
	if err != nil {
		return "", err
	}
	if ret.C == 0 {
		return period.label + "暂无盲盒记录", nil
	}
	count := fmt.Sprintf("%s共%d个", period.label, ret.C)
	switch {
	case ret.R > 0:
		return fmt.Sprintf("%s，赚%.2f元", count, float64(ret.R)/1000), nil
	case ret.R < 0:
		return fmt.Sprintf("%s，亏%.2f元", count, math.Abs(float64(ret.R)/1000)), nil
	default:
		return count + "，不赚不亏", nil
	}
}
