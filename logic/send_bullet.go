package logic

import (
	"context"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/companion"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/entity"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/http"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/svc"
	"github.com/zeromicro/go-zero/core/logx"
	"time"
)

var sender = &BulletSender{bulletChan: make(chan entity.Bullet, 1000)}

type BulletSender struct {
	bulletChan chan entity.Bullet
}

func PushToBulletSender(msg string, reply ...*entity.DanmuMsgTextReplyInfo) {
	logx.Info("PushToBulletSender成功", msg)
	bullet := entity.Bullet{
		Msg:   msg,
		Reply: reply,
	}
	sender.bulletChan <- bullet
}

// PushPersona keeps generated messages as one bounded, expiring utterance.
func PushPersona(msg string, valid func() bool, reply ...*entity.DanmuMsgTextReplyInfo) {
	if sender == nil {
		return
	}
	select {
	case sender.bulletChan <- entity.Bullet{Msg: msg, Reply: reply, Valid: valid}:
	default:
		logx.Error("persona send queue full; discarded")
	}
}

func StartSendBullet(ctx context.Context, svcCtx *svc.ServiceContext) {

	var msg string
	for {
		select {
		case <-ctx.Done():
			goto END
		case bullet := <-sender.bulletChan:
			if bullet.Valid != nil && !bullet.Valid() {
				continue
			}
			msg = bullet.Msg
			if svcCtx.Config.Companion.Enabled {
				name := ""
				if len(bullet.Reply) > 0 && bullet.Reply[0] != nil {
					name = bullet.Reply[0].ReplyName
				}
				if len(bullet.Reply) > 0 && name == "" {
					logx.Error("skip: reply name unavailable for length accounting")
					continue
				}
				msg = companion.Fit(msg, name, svcCtx.Config.Companion.MaxUnits)
				if msg == "" {
					continue
				}
				if err := http.SendWithGuard(msg, svcCtx, bullet.Valid, bullet.Reply...); err != nil {
					logx.Errorf("弹幕发送失败: %v", err)
				} else {
					svcCtx.Companion.Sent(msg, time.Now())
					logx.Infof("弹幕发送成功：%s", msg)
				}
				time.Sleep(time.Second)
				continue
			}
			var danmuLen = svcCtx.Config.DanmuLen
			var msgdata []string
			msgrun := []rune(msg)
			msgLen := len(msgrun)
			msgcount := msgLen / danmuLen
			tmpmsgcount := msgLen % danmuLen
			if tmpmsgcount != 0 {
				msgcount += 1
			}
			for m := 1; m <= msgcount; m++ {
				if msgLen < m*danmuLen {
					msgdata = append(msgdata, string(msgrun[(m-1)*danmuLen:msgLen]))
					continue
				}
				msgdata = append(msgdata, string(msgrun[(m-1)*danmuLen:danmuLen*m]))
			}
			for _, msgs := range msgdata {
				if err := http.Send(msgs, svcCtx, bullet.Reply...); err != nil {
					logx.Errorf("弹幕发送失败：%s msg: %s", err, msgs)
				} else {
					logx.Infof("弹幕发送成功：%s", msgs)
				}
				//fmt.Println(msgs)
				time.Sleep(1 * time.Second) // 防止弹幕发送过快
			}
		}

	}
END:
}
