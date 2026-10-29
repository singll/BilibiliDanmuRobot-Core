package companion

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/config"
)

func testRuntime() *Runtime {
	return New(config.CompanionConfig{Idle: true, IdleAfterSeconds: 300, IdleCooldownSeconds: 900, IdleMaxPerHour: 2, QuietAfterSendSeconds: 120}, nil, nil)
}
func TestIdleRequiresHealthyQuietRoom(t *testing.T) {
	r := testRuntime()
	start := time.Now()
	r.SetLive(true, start)
	now := start.Add(6 * time.Minute)
	if r.CanIdle(now) {
		t.Fatal("stale connection counted as silence")
	}
	r.Packet(start.Add(time.Minute))
	r.Packet(start.Add(2 * time.Minute))
	r.Packet(start.Add(3 * time.Minute))
	r.Packet(start.Add(4 * time.Minute))
	r.Packet(start.Add(5 * time.Minute))
	r.Packet(now)
	if !r.CanIdle(now) {
		t.Fatal("quiet room should be eligible")
	}
	r.Activity(now)
	if r.CanIdle(now) {
		t.Fatal("new chat should cancel")
	}
}
func TestQueuedMessageInvalidatedByActivityAndOffline(t *testing.T) {
	r := testRuntime()
	now := time.Now()
	r.SetLive(true, now)
	ticket := Ticket{Epoch: r.epoch, Activity: r.activity, Expires: now.Add(time.Minute), Idle: true}
	if !r.Valid(ticket, now) {
		t.Fatal("ticket")
	}
	r.Activity(now)
	if r.Valid(ticket, now) {
		t.Fatal("activity failed to cancel idle")
	}
	ticket.Idle = false
	if !r.Valid(ticket, now) {
		t.Fatal("activity should preserve explicit replies")
	}
	r.SetLive(false, now)
	r.SetLive(true, now)
	if r.Valid(ticket, now) {
		t.Fatal("old stream ticket survived")
	}
}
func TestWelcomeDedup(t *testing.T) {
	r := testRuntime()
	now := time.Now()
	if !r.AdmitWelcome(1, now) || r.AdmitWelcome(1, now.Add(10*time.Second)) || !r.AdmitWelcome(1, now.Add(time.Minute)) {
		t.Fatal("dedup")
	}
}
func TestGenerateFailureUsesFittedFallback(t *testing.T) {
	r := New(config.CompanionConfig{}, func(context.Context, Event) (string, error) { return "", errors.New("offline") }, nil)
	now := time.Now()
	r.SetLive(true, now)
	var got string
	r.process(context.Background(), delivery{Event{Scene: "gift", Name: "鱼清悦", Fallback: "谢谢你的礼物💙"}, Ticket{Epoch: r.epoch, Expires: now.Add(time.Minute)}, func(s string, v func() bool) {
		if !v() {
			t.Fatal("invalid fallback")
		}
		got = s
	}})
	if got != "谢谢你的礼物💙" {
		t.Fatal(got)
	}
}
func TestActivityDuringGenerationCancelsIdle(t *testing.T) {
	r := testRuntime()
	now := time.Now()
	r.SetLive(true, now)
	sent := false
	r.generate = func(context.Context, Event) (string, error) { r.Activity(time.Now()); return "在呢", nil }
	r.process(context.Background(), delivery{Event{Scene: "idle"}, Ticket{Epoch: r.epoch, Activity: r.activity, Expires: now.Add(time.Minute), Idle: true}, func(string, func() bool) { sent = true }})
	if sent {
		t.Fatal("stale idle enqueued")
	}
}
func TestSameQuietPeriodOnlyOnce(t *testing.T) {
	r := testRuntime()
	now := time.Now()
	r.SetLive(true, now.Add(-6*time.Minute))
	r.lastPacket = now
	if !r.submit(Event{Scene: "idle"}, time.Minute, func(string, func() bool) {}, now) {
		t.Fatal("first")
	}
	if r.submit(Event{Scene: "idle"}, time.Minute, func(string, func() bool) {}, now) {
		t.Fatal("repeated idle")
	}
}
func TestConcurrentActivity(t *testing.T) {
	r := testRuntime()
	r.SetLive(true, time.Now())
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				now := time.Now()
				r.Packet(now)
				r.Activity(now)
				r.Sent("好", now)
				r.CanIdle(now)
			}
		}()
	}
	wg.Wait()
}

func TestIdleCooldownAndHourlyLimit(t *testing.T) {
	r := testRuntime()
	now := time.Now()
	r.SetLive(true, now.Add(-10*time.Minute))
	r.lastPacket = now
	for i := 0; i < 2; i++ {
		moment := now.Add(time.Duration(i) * 20 * time.Minute)
		r.lastPacket = moment
		r.idleAt = moment.Add(-time.Second)
		r.idleAttempted = false
		if !r.submit(Event{Scene: "idle"}, time.Minute, func(string, func() bool) {}, moment) {
			t.Fatal("eligible attempt", i)
		}
		<-r.low
		r.idleAttempted = false
		if r.CanIdle(moment.Add(time.Second)) {
			t.Fatal("cooldown bypassed")
		}
	}
	r.lastPacket = now.Add(40 * time.Minute)
	r.idleAt = now
	if r.CanIdle(now.Add(40 * time.Minute)) {
		t.Fatal("hourly cap bypassed")
	}
	r.lastPacket = now.Add(61 * time.Minute)
	if !r.CanIdle(now.Add(61 * time.Minute)) {
		t.Fatal("hourly quota did not recover")
	}
}
func TestCancelledWorkerCannotDeliver(t *testing.T) {
	r := New(config.CompanionConfig{}, func(ctx context.Context, e Event) (string, error) { <-ctx.Done(); return "", ctx.Err() }, nil)
	r.SetLive(true, time.Now())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sent := false
	r.process(ctx, delivery{Event{Fallback: "fallback"}, Ticket{Epoch: r.epoch, Expires: time.Now().Add(time.Minute)}, func(string, func() bool) { sent = true }})
	if sent {
		t.Fatal("worker delivered after cancellation")
	}
}
