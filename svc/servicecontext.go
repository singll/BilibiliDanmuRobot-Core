package svc

import (
	"context"
	"fmt"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/companion"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/config"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config            *config.Config
	Companion         *companion.Runtime
	OtherSideUid      map[int64]bool
	SignInModel       model.SignInModel
	DanmuCntModel     model.DanmuCntModel
	BlindBoxStatModel model.BlindBoxStatModel
	UserID            int64 //主播id
	Autointerract     struct {
		EntryEffect        bool
		WelcomeHighWealthy bool
		InteractWord       bool
	}
	RobotID string //机器人uid
}

func NewServiceContext(c config.Config) *ServiceContext {
	dbFile := fmt.Sprintf("%s/%s?_pragma=busy_timeout(5000)", c.DBPath, c.DBName)
	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	runtime := companion.New(c.Companion, func(ctx context.Context, e companion.Event) (string, error) {
		return companion.Generate(ctx, c.ChatGPT, c.Companion.MaxTokens, e)
	}, func(s string) { logx.Info(s) })
	return &ServiceContext{
		Companion:         runtime,
		OtherSideUid:      make(map[int64]bool),
		SignInModel:       model.NewSignInModel(db, int64(c.RoomId)),
		DanmuCntModel:     model.NewDanmuCntModel(db, int64(c.RoomId)),
		BlindBoxStatModel: model.NewBlindBoxStatModel(db, int64(c.RoomId)),
		Config:            &c,
		UserID:            0,
	}
}
