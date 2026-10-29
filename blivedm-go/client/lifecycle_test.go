package client

import (
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/blivedm-go/message"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/blivedm-go/packet"
	"testing"
)

func TestOrderedStreamLifecycleAndStop(t *testing.T) {
	c := NewClient(1)
	connected, live, packets := false, false, 0
	c.OnConnection = func(ok bool) { connected = ok }
	c.OnPacket = func() { packets++ }
	c.OnLiveStart(func(*message.LiveStart) { live = true })
	c.OnLiveStop(func(*message.LiveStop) { live = false })
	c.Handle(packet.Packet{Operation: packet.RoomEnterResponse, Body: []byte(`{"code":0}`)})
	if !connected {
		t.Fatal("missing auth notification")
	}
	c.Handle(packet.Packet{Operation: packet.Notification, Body: []byte(`{"cmd":"LIVE"}`)})
	c.Handle(packet.Packet{Operation: packet.Notification, Body: []byte(`{"cmd":"PREPARING"}`)})
	if live {
		t.Fatal("events were not handled in order")
	}
	c.Handle(packet.Packet{Operation: packet.RoomEnterResponse, Body: []byte(`{"code":-1}`)})
	if connected {
		t.Fatal("rejected auth counted as healthy")
	}
	c.Stop()
	c.Handle(packet.Packet{Operation: packet.RoomEnterResponse, Body: []byte(`{"code":0}`)})
	if connected || packets != 4 {
		t.Fatal("stopped socket reactivated runtime")
	}
}
