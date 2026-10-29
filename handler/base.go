package handler

import (
	"context"
	"errors"
	"fmt"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/blivedm-go/client"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/blivedm-go/message"
	_ "github.com/Hello-Moeka/BilibiliDanmuRobot-Core/blivedm-go/utils"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/companion"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/config"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/entity"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/http"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/logic"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/logic/danmu"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/svc"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/utiles"
	_ "github.com/glebarez/go-sqlite"
	"github.com/robfig/cron/v3"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"math/rand"
	"os"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type wsHandler struct {
	client *client.Client
	svc    *svc.ServiceContext
	// 机器人
	robotBulletCtx    context.Context
	robotBulletCancel context.CancelFunc
	// 弹幕发送
	sendBulletCtx    context.Context
	sendBulletCancel context.CancelFunc
	// 特效欢迎
	ineterractCtx    context.Context
	ineterractCancel context.CancelFunc
	//礼物感谢
	thanksGiftCtx   context.Context
	thankGiftCancel context.CancelFunc
	//pk提醒
	pkCtx    context.Context
	pkCancel context.CancelFunc
	//弹幕处理
	danmuLogicCtx    context.Context
	danmuLogicCancel context.CancelFunc
	//定时弹幕
	corndanmu           *cron.Cron
	mapCronDanmuSendIdx map[int]int
	userId              int
	initStart           bool
	logicOnce           sync.Once
	clientGeneration    atomic.Uint64
	companionCtx        context.Context
	companionCancel     context.CancelFunc
}

var ConfigFile = "etc/bilidanmaku-api.yaml"

func NewWsHandler() WsHandler {
	ctx, err := mustloadConfig()
	if err != nil {
		return nil
	}
	ws := new(wsHandler)
	ws.initStart = false
	err = ws.starthttp()
	if err != nil {
		logx.Error(err)
		return nil
	}
	ws.client = client.NewClient(ctx.Config.RoomId)
	ws.client.SetCookie(http.CookieStr)
	ws.svc = ctx
	//初始化定时弹幕
	ws.corndanmu = cron.New(cron.WithParser(cron.NewParser(
		cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
	)))
	ws.mapCronDanmuSendIdx = make(map[int]int)

	// 设置uid作为基本配置
	strUserId, ok := http.CookieList["DedeUserID"]
	if !ok {
		logx.Infof("uid加载失败，请重新登录")
		return nil
	}
	ws.userId, err = strconv.Atoi(strUserId)
	ctx.RobotID = strUserId
	roominfo, err := http.RoomInit(ctx.Config.RoomId)
	if err != nil {
		logx.Error(err)
		//return nil
	}
	ctx.UserID = roominfo.Data.Uid
	return ws
}
func (ws *wsHandler) ReloadConfig() error {
	ctx, err := mustloadConfig()
	oldconfig := *ws.svc.Config
	if err != nil {
		return err
	}
	if ctx.Config.Companion != oldconfig.Companion || ctx.Config.ChatGPT != oldconfig.ChatGPT {
		return errors.New("人格或模型配置已保存，请重启机器人使其生效")
	}
	ws.svc.Config = ctx.Config
	if ctx.Config.RoomId != oldconfig.RoomId {
		logx.Infof("房间号更改，更换房间号 ：%v", ctx.Config.RoomId)
		ws.StopWsClient()
		if err := ws.StartWsClient(); err != nil {
			return err
		}
	}
	if ctx.Config.CronDanmu != oldconfig.CronDanmu || !areSlicesEqual(ctx.Config.CronDanmuList, oldconfig.CronDanmuList) {
		logx.Info("识别到定时弹幕配置发生变化，重新加载")
		for _, i := range ws.corndanmu.Entries() {
			ws.corndanmu.Remove(i.ID)
		}
		ws.corndanmuStart()
	}
	return nil
}

type WsHandler interface {
	InitStartWsClient()
	StopWsClient()
	SayGoodbye()
	StopChanel()
	StartWsClient() error
	starthttp() error
	ReloadConfig() error
	GetSvc() svc.ServiceContext
	GetUserinfo() *entity.UserinfoLite
}

func (w *wsHandler) InitStartWsClient() {
	w.logicOnce.Do(w.startLogic)
}
func (w *wsHandler) StartWsClient() error {
	w.InitStartWsClient()

	if w.svc.Config.EntryMsg != "off" {
		err := http.Send(w.svc.Config.EntryMsg, w.svc)
		if err != nil {
			logx.Error(err)
		}
	}
	w.corndanmu.Start()
	w.client = client.NewClient(w.svc.Config.RoomId)
	w.client.SetCookie(http.CookieStr)
	id := w.clientGeneration.Add(1)
	var live, connected atomic.Bool
	live.Store(true)
	update := func() {
		if w.clientGeneration.Load() == id {
			w.svc.Companion.SetLive(live.Load() && connected.Load(), time.Now())
		}
	}
	w.client.OnPacket = func() {
		if w.clientGeneration.Load() == id {
			w.svc.Companion.Packet(time.Now())
		}
	}
	w.client.OnConnection = func(ok bool) { connected.Store(ok); update() }
	w.client.OnLiveStop(func(_ *message.LiveStop) { live.Store(false); update() })
	w.client.OnLiveStart(func(_ *message.LiveStart) { live.Store(true); update() })
	w.registerHandler()
	return w.client.Start()
}
func (w *wsHandler) GetUserinfo() *entity.UserinfoLite {
	return http.GetUserInfo()
}
func (w *wsHandler) GetSvc() svc.ServiceContext {
	return *w.svc
}
func (w *wsHandler) StopWsClient() {
	w.clientGeneration.Add(1)
	w.svc.Companion.SetLive(false, time.Now())
	w.corndanmu.Stop()
	w.client.Stop()
	//w.svc.Db.Db.Close()
}
func (w *wsHandler) StopChanel() {
	w.svc.Companion.SetLive(false, time.Now())
	if w.companionCancel != nil {
		w.companionCancel()
	}
	if w.sendBulletCancel != nil {
		w.sendBulletCancel()
	}
	if w.robotBulletCancel != nil {
		w.robotBulletCancel()
	}
	if w.thankGiftCancel != nil {
		w.thankGiftCancel()
	}
	if w.ineterractCancel != nil {
		w.ineterractCancel() // 关闭弹幕姬goroutine
	}
	if w.pkCancel != nil {
		w.pkCancel()
	}
	if w.danmuLogicCancel != nil {
		w.danmuLogicCancel()
	}
	for _, i := range w.corndanmu.Entries() {
		w.corndanmu.Remove(i.ID)
	}
}
func (w *wsHandler) SayGoodbye() {
	if len(w.svc.Config.GoodbyeInfo) > 0 {

		var danmuLen = w.svc.Config.DanmuLen
		var msgdata []string
		msgrun := []rune(w.svc.Config.GoodbyeInfo)
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
			err := http.Send(msgs, w.svc)
			if err != nil {
				logx.Errorf("下播弹幕发送失败：%s msg: %s", err, msgs)
			}
			time.Sleep(1 * time.Second) // 防止弹幕发送过快
			// logx.Info(">>>>>>>>>", msgs)
		}
	}
}
func (w *wsHandler) startLogic() {
	w.companionCtx, w.companionCancel = context.WithCancel(context.Background())
	w.svc.Companion.Start(w.companionCtx)
	if w.svc.Config.Companion.Enabled && w.svc.Config.Companion.Idle {
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-w.companionCtx.Done():
					return
				case now := <-ticker.C:
					if w.svc.Companion.CanIdle(now) {
						w.svc.Companion.Submit(companion.Event{Scene: "idle", Facts: "直播中，近期没有观众弹幕或礼物互动。温柔陪伴大家，可以围绕喜欢鱼清悦或一个轻松的小话题说一句。不声称看到画面，不声称回忆过去。"}, 30*time.Second, func(s string, valid func() bool) { logic.PushPersona(s, valid) })
					}
				}
			}
		}()
	}

	w.sendBulletCtx, w.sendBulletCancel = context.WithCancel(context.Background())
	go logic.StartSendBullet(w.sendBulletCtx, w.svc)
	logx.Info("弹幕推送已开启...")
	// 机器人
	w.robotBulletCtx, w.robotBulletCancel = context.WithCancel(context.Background())
	go logic.StartBulletRobot(w.robotBulletCtx, w.svc)
	// 弹幕逻辑
	w.danmuLogicCtx, w.danmuLogicCancel = context.WithCancel(context.Background())
	go danmu.StartDanmuLogic(w.danmuLogicCtx, w.svc)

	logx.Info("弹幕机器人已开启")
	// 特效欢迎
	w.ineterractCtx, w.ineterractCancel = context.WithCancel(context.Background())
	go logic.Interact(w.ineterractCtx, w.svc)

	logx.Info("欢迎模块已开启")

	// 礼物感谢
	w.thanksGiftCtx, w.thankGiftCancel = context.WithCancel(context.Background())
	go logic.ThanksGift(w.thanksGiftCtx, w.svc)

	logx.Info("礼物感谢已开启")
	// pk提醒
	w.pkCtx, w.pkCancel = context.WithCancel(context.Background())
	go logic.PK(w.pkCtx, w.svc)

	// 下播提醒
	// w.sayGoodbyeByWs()

	//定时弹幕
	w.corndanmuStart()

	//w.registerHandler()
}
func (w *wsHandler) registerHandler() {

	w.welcomeEntryEffect()
	w.welcomeInteractWord()
	logx.Info("弹幕处理已开启")
	w.receiveDanmu()
	// 天选自动关闭欢迎
	w.anchorLot()
	logx.Info("pk提醒已开启")
	w.pkBattleStart()
	w.pkBattleEnd()
	// 禁言用户提醒
	w.blockUser()
	w.thankGifts()
	// 红包
	w.redPocket()
}
func (w *wsHandler) starthttp() error {
	var err error
	http.InitHttpClient()
	// 判断是否存在历史cookie
	if http.FileExists("token/bili_token.txt") && http.FileExists("token/bili_token.json") {
		err = http.SetHistoryCookie()
		if err != nil {
			logx.Error("用户登录失败")
			return err
		}
		logx.Info("用户登录成功")
	} else {
		//if err = w.userlogin(); err != nil {
		//	logx.Errorf("用户登录失败：%v", err)
		//	return
		//}
		//logx.Info("用户登录成功")
		logx.Error("用户登录失败")
		return errors.New("用户登录失败")
	}
	return nil
}
func (w *wsHandler) userlogin() error {
	var err error
	http.InitHttpClient()
	var loginUrl *entity.LoginUrl
	if loginUrl, err = http.GetLoginUrl(); err != nil {
		logx.Error(err)
		return err
	}

	if err = utiles.GenerateQr(loginUrl.Data.Url); err != nil {
		logx.Error(err)
		return err
	}

	if _, err = http.GetLoginInfo(loginUrl.Data.OauthKey); err != nil {
		logx.Error(err)
		return err
	}

	return err
}
func (w *wsHandler) corndanmuStart() {
	if w.svc.Config.CronDanmu == false {
		return
	}
	for n, danmux := range w.svc.Config.CronDanmuList {
		if danmux.Danmu != nil {
			i := n
			danmus := danmux
			_, err := w.corndanmu.AddFunc(danmus.Cron, func() {
				if w.svc.Config.Companion.Enabled && w.svc.Config.Companion.Clock {
					fallback := ""
					if len(danmus.Danmu) > 0 {
						fallback = danmus.Danmu[rand.Intn(len(danmus.Danmu))]
					}
					w.svc.Companion.Submit(companion.Event{Scene: "clock", Facts: "请按提供的北京时间报时，随后附上一句符合人设的简短关心。原时段文案参考：" + fallback, Fallback: fallback}, 45*time.Second, func(s string, valid func() bool) { logic.PushPersona(s, valid) })
					return
				}
				if len(danmus.Danmu) > 0 {
					if danmus.Random {
						logic.PushToBulletSender(danmus.Danmu[rand.Intn(len(danmus.Danmu))])
					} else {
						_, ok := w.mapCronDanmuSendIdx[i]
						if !ok {
							w.mapCronDanmuSendIdx[i] = 0
						}
						w.mapCronDanmuSendIdx[i] = w.mapCronDanmuSendIdx[i] + 1
						logic.PushToBulletSender(danmus.Danmu[w.mapCronDanmuSendIdx[i]%len(danmus.Danmu)])
					}
				}
			})
			if err != nil {
				logx.Errorf("第%d条定时弹幕配置出现错误: %v", i+1, err)
			}
		}
	}
	w.corndanmu.Start()
}
func mustloadConfig() (*svc.ServiceContext, error) {
	dir := "./token"
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// Directory does not exist, create it
		err = os.Mkdir(dir, 0755)
		if err != nil {
			panic(fmt.Sprintf("无法创建token文件夹 请手动创建:%s", err))
		}
	}

	var c config.Config
	conf.MustLoad(ConfigFile, &c, conf.UseEnv())
	if err := c.ValidateCompanion(); err != nil {
		return nil, err
	}
	logx.MustSetup(c.Log)
	logx.DisableStat()
	//配置数据库文件夹
	info, err := os.Stat(c.DBPath)
	if os.IsNotExist(err) || !info.IsDir() {
		err = os.MkdirAll(c.DBPath, 0777)
		if err != nil {
			logx.Errorf("文件夹创建失败：%s", c.DBPath)
			return nil, err
		}
	}
	ctx := svc.NewServiceContext(c)
	return ctx, err
}

// 比较两个 Person 切片是否相同
func areSlicesEqual(a, b []config.CronDanmuList) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !reflect.DeepEqual(a[i], b[i]) {
			return false
		}
	}

	return true
}
