package http

import (
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/companion"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/config"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/entity"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/svc"
	"io"
	"net/http"
	"strings"
	"testing"
)

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestSendFullDisplayBudgetAndGuard(t *testing.T) {
	old := http.DefaultClient
	defer func() { http.DefaultClient = old }()
	calls := 0
	http.DefaultClient = &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		if companion.Units("@鱼清悦 "+r.FormValue("msg")) > 80 || strings.ContainsAny(r.FormValue("msg"), "\n*`") {
			t.Fatal("bad display text")
		}
		if r.FormValue("reply_mid") != "42" {
			t.Fatal("missing native reply")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"code":0,"msg":""}`)), Header: make(http.Header)}, nil
	})}
	c := &svc.ServiceContext{Config: &config.Config{RoomId: 1, Companion: config.CompanionConfig{Enabled: true, MaxUnits: 80}}}
	reply := &entity.DanmuMsgTextReplyInfo{ReplyUid: "42", ReplyName: "鱼清悦"}
	if err := SendWithGuard("**"+strings.Repeat("喜欢清悦💙", 30)+"**\n", c, func() bool { return true }, reply); err != nil {
		t.Fatal(err)
	}
	if err := SendWithGuard("过时消息", c, func() bool { return false }, reply); err == nil {
		t.Fatal("missing cancellation error")
	}
	if calls != 1 {
		t.Fatal("cancelled message reached network")
	}
}
