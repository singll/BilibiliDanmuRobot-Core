package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/companion"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/entity"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/svc"
	"github.com/avast/retry-go/v4"
	"github.com/zeromicro/go-zero/core/logx"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"
)

//func GetDanmuInfo(svcCtx *svc.ServiceContext) (*entity.ResponseBulletInfo, error) {
//	var err error
//	var resp *resty.Response
//	var url = "https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo?id=" + strconv.Itoa(svcCtx.Config.RoomId) + "&type=0"
//	r := &entity.ResponseBulletInfo{}
//	if resp, err = cli.R().
//		SetHeader("user-agent", userAgent).
//		Get(url); err != nil {
//		logx.Error("请求getDanmuInfo失败：", err)
//		return nil, err
//	}
//	if err = json.Unmarshal(resp.Body(), r); err != nil {
//		logx.Error("Unmarshal失败：", err, "body:", string(resp.Body()))
//		return nil, err
//	}
//
//	return r, nil
//}

func Send(msg string, svcCtx *svc.ServiceContext, reply ...*entity.DanmuMsgTextReplyInfo) error {
	return SendWithGuard(msg, svcCtx, nil, reply...)
}

func SendWithGuard(msg string, svcCtx *svc.ServiceContext, valid func() bool, reply ...*entity.DanmuMsgTextReplyInfo) error {
	if svcCtx.Config.Companion.Enabled {
		name := ""
		if len(reply) > 0 && reply[0] != nil {
			name = reply[0].ReplyName
			if name == "" {
				return errors.New("missing mention name")
			}
		}
		msg = companion.Fit(msg, name, svcCtx.Config.Companion.MaxUnits)
		if msg == "" {
			return errors.New("empty or over-budget message")
		}
	}
	var err error
	var url = "https://api.live.bilibili.com/msg/send"
	var respdata *entity.DanmuResp = new(entity.DanmuResp)
	m := make(map[string]string)
	m["bubble"] = "5"
	m["msg"] = msg
	m["color"] = "4546550"
	//m["mode"] = "4"
	m["fontsize"] = "25"
	m["rnd"] = strconv.FormatInt(time.Now().Unix(), 10)
	//m["room_type"] = "0"
	//m["jumpfrom"] = "0"
	//m["reply_attr"] = "0"
	if len(reply) > 0 && reply[0] != nil {
		m["reply_mid"] = reply[0].ReplyUid
		if len(reply[0].ReplyMsgId) > 0 {
			m["replay_dmid"] = reply[0].ReplyMsgId
		}
	}
	m["roomid"] = strconv.Itoa(svcCtx.Config.RoomId)
	m["csrf"] = CookieList["bili_jct"]
	m["csrf_token"] = CookieList["bili_jct"]
	err = retry.Do(func() error {
		if valid != nil && !valid() {
			return retry.Unrecoverable(errors.New("message expired or cancelled"))
		}
		_, data, err := postWithFormData(http.MethodPost, url, userAgent, CookieStr, &m)
		if err != nil {
			logx.Errorf("请求send失败：%v", err)
			return err
		}
		err = json.Unmarshal(data, respdata)
		if err != nil {
			logx.Errorf("send弹幕响应解析失败:%v", err)
			return err
		}
		if respdata.Code != 0 {
			logx.Infof("请求send失败:%s", respdata.Msg)
			return errors.New(respdata.Msg)
		}
		if respdata.Msg == "f" {
			return errors.New("弹幕被服务器拒绝")
		}
		return nil
	}, retry.Attempts(3), retry.Delay(1*time.Second))
	if err != nil {
		logx.Error(err)
	}
	return err
}
func postWithFormData(method, url, ua, cookie string, postData *map[string]string) (int, []byte, error) {
	body := new(bytes.Buffer)
	w := multipart.NewWriter(body)
	for k, v := range *postData {
		w.WriteField(k, v)
	}
	w.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Cookie", cookie)
	req.Header.Set("user-agent", ua)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logx.Error(err)
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil, fmt.Errorf("send HTTP %d", resp.StatusCode)
	}
	return resp.StatusCode, data, nil
}
