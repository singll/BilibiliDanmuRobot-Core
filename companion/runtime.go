package companion

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/config"
)

type Ticket struct {
	Epoch    uint64
	Activity uint64
	Expires  time.Time
	Idle     bool
}

type delivery struct {
	event  Event
	ticket Ticket
	send   func(string, func() bool)
}

// Runtime serializes model work while keeping socket/event handlers responsive.
// Activity and delivery tickets invalidate stale work after reconnect or activity.
type Runtime struct {
	mu                                           sync.Mutex
	cfg                                          config.CompanionConfig
	generate                                     GenerateFunc
	log                                          func(string)
	busy                                         bool
	live                                         bool
	epoch                                        uint64
	activity                                     uint64
	lastPacket, lastActivity, lastSent, lastIdle time.Time
	nextRequest                                  time.Time
	idleAt                                       time.Time
	idleAttempted                                bool
	idleTimes                                    []time.Time
	recent                                       []string
	welcomed                                     map[int64]time.Time
	high, low                                    chan delivery
	life                                         context.Context
	cancel                                       context.CancelFunc
}

func New(cfg config.CompanionConfig, gen GenerateFunc, log func(string)) *Runtime {
	if cfg.MaxUnits <= 0 || cfg.MaxUnits > 80 {
		cfg.MaxUnits = 80
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 12
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 256
	}
	if cfg.IdleAfterSeconds <= 0 {
		cfg.IdleAfterSeconds = 300
	}
	if cfg.IdleCooldownSeconds <= 0 {
		cfg.IdleCooldownSeconds = 900
	}
	if cfg.IdleMaxPerHour <= 0 {
		cfg.IdleMaxPerHour = 2
	}
	if cfg.QuietAfterSendSeconds <= 0 {
		cfg.QuietAfterSendSeconds = 120
	}
	if cfg.WelcomeCooldownSeconds <= 0 {
		cfg.WelcomeCooldownSeconds = 60
	}
	if log == nil {
		log = func(string) {}
	}
	return &Runtime{cfg: cfg, generate: gen, log: log, welcomed: make(map[int64]time.Time), high: make(chan delivery, 32), low: make(chan delivery, 16)}
}
func (r *Runtime) Start(ctx context.Context) { go r.worker(ctx) }

// Session lets aggregators discard input retained across a disconnect or stream.
func (r *Runtime) Session(now time.Time) (uint64, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.epoch, r.healthy(now)
}
func (r *Runtime) SetLive(live bool, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.live == live {
		return
	}
	if r.cancel != nil {
		r.cancel()
	}
	r.live = live
	r.epoch++
	r.activity++
	r.idleAttempted = false
	r.lastActivity = now
	r.lastPacket = now
	r.idleAt = r.idleDeadline(now)
	if live {
		r.life, r.cancel = context.WithCancel(context.Background())
		r.welcomed = make(map[int64]time.Time)
	}
}
func (r *Runtime) Packet(now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.lastPacket.IsZero() && now.Sub(r.lastPacket) > 75*time.Second {
		r.activity++
		r.lastActivity = now
		r.idleAttempted = false
		r.idleAt = r.idleDeadline(now)
	}
	r.lastPacket = now
}
func (r *Runtime) Activity(now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastActivity = now
	r.activity++
	r.idleAttempted = false
	r.idleAt = r.idleDeadline(now)
}
func (r *Runtime) Sent(msg string, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastSent = now
	r.recent = append(r.recent, msg)
	if len(r.recent) > 8 {
		r.recent = r.recent[len(r.recent)-8:]
	}
}
func (r *Runtime) idleDeadline(now time.Time) time.Time {
	j := 0
	if r.cfg.IdleJitterSeconds > 0 {
		j = rand.Intn(r.cfg.IdleJitterSeconds + 1)
	}
	return now.Add(time.Duration(r.cfg.IdleAfterSeconds+j) * time.Second)
}
func (r *Runtime) healthy(now time.Time) bool {
	return r.live && !r.lastPacket.IsZero() && now.Sub(r.lastPacket) < 75*time.Second
}
func (r *Runtime) Valid(t Ticket, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.valid(t, now)
}
func (r *Runtime) valid(t Ticket, now time.Time) bool {
	if !r.healthy(now) || t.Epoch != r.epoch || now.After(t.Expires) {
		return false
	}
	if t.Idle && (t.Activity != r.activity || now.Sub(r.lastSent) < time.Duration(r.cfg.QuietAfterSendSeconds)*time.Second) {
		return false
	}
	return true
}
func (r *Runtime) AdmitWelcome(uid int64, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	d := time.Duration(r.cfg.WelcomeCooldownSeconds) * time.Second
	if last, ok := r.welcomed[uid]; ok && now.Sub(last) < d {
		return false
	}
	for id, t := range r.welcomed {
		if now.Sub(t) > d {
			delete(r.welcomed, id)
		}
	}
	r.welcomed[uid] = now
	return true
}
func (r *Runtime) CanIdle(now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.canIdle(now)
}
func (r *Runtime) canIdle(now time.Time) bool {
	if !r.cfg.Idle || !r.healthy(now) || r.idleAttempted || now.Before(r.idleAt) || now.Sub(r.lastSent) < time.Duration(r.cfg.QuietAfterSendSeconds)*time.Second || now.Sub(r.lastIdle) < time.Duration(r.cfg.IdleCooldownSeconds)*time.Second || r.busy || len(r.high) > 0 || len(r.low) > 0 {
		return false
	}
	recent := r.idleTimes[:0]
	for _, t := range r.idleTimes {
		if now.Sub(t) < time.Hour {
			recent = append(recent, t)
		}
	}
	r.idleTimes = recent
	return len(recent) < r.cfg.IdleMaxPerHour
}
func (r *Runtime) Submit(e Event, ttl time.Duration, send func(string, func() bool)) bool {
	return r.submit(e, ttl, send, time.Now())
}
func (r *Runtime) submit(e Event, ttl time.Duration, send func(string, func() bool), now time.Time) bool {
	r.mu.Lock()
	if !r.healthy(now) {
		r.mu.Unlock()
		return false
	}
	idle := e.Scene == "idle"
	if idle && !r.canIdle(now) {
		r.mu.Unlock()
		return false
	}
	if idle {
		r.idleAttempted = true
		r.lastIdle = now
		r.idleTimes = append(r.idleTimes, now)
	}
	t := Ticket{Epoch: r.epoch, Activity: r.activity, Expires: now.Add(ttl), Idle: idle}
	e.Budget = ReplyBudget(r.cfg.MaxUnits, e.Name)
	e.Time = LocalTime(now)
	e.Recent = append([]string(nil), r.recent...)
	r.mu.Unlock()
	if e.Budget < 2 {
		r.log("skip: mention exhausts reply budget")
		return false
	}
	job := delivery{e, t, send}
	queue := r.high
	if e.Scene == "idle" || e.Scene == "clock" {
		queue = r.low
	}
	select {
	case queue <- job:
		return true
	default:
		r.log("skip: persona queue full")
		return false
	}
}
func (r *Runtime) worker(ctx context.Context) {
	for {
		var job delivery
		select {
		case <-ctx.Done():
			return
		case job = <-r.high:
		default:
			select {
			case <-ctx.Done():
				return
			case job = <-r.high:
			case job = <-r.low:
			}
		}
		r.mu.Lock()
		r.busy = true
		r.mu.Unlock()
		r.process(ctx, job)
		r.mu.Lock()
		r.busy = false
		r.mu.Unlock()
	}
}
func (r *Runtime) process(parent context.Context, j delivery) {
	if parent.Err() != nil || !r.Valid(j.ticket, time.Now()) {
		return
	}
	r.mu.Lock()
	wait := time.Until(r.nextRequest)
	life := r.life
	r.mu.Unlock()
	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
	waiting:
		for {
			select {
			case <-parent.Done():
				return
			case <-life.Done():
				return
			case <-timer.C:
				break waiting
			case <-tick.C:
				if parent.Err() != nil || !r.Valid(j.ticket, time.Now()) {
					return
				}
				if (j.event.Scene == "idle" || j.event.Scene == "clock") && len(r.high) > 0 {
					select {
					case r.low <- j:
					default:
						r.log("skip low priority event: queue full")
					}
					return
				}
			}
		}
	}
	if parent.Err() != nil || !r.Valid(j.ticket, time.Now()) {
		return
	}
	r.mu.Lock()
	r.nextRequest = time.Now().Add(time.Duration(r.cfg.MinRequestSeconds) * time.Second)
	r.mu.Unlock()
	ctx, cancel := context.WithTimeout(life, time.Duration(r.cfg.TimeoutSeconds)*time.Second)
	defer cancel()
	stop := context.AfterFunc(parent, cancel)
	defer stop()
	out, err := r.generate(ctx, j.event)
	if parent.Err() != nil || !r.Valid(j.ticket, time.Now()) {
		return
	}
	if err != nil {
		r.log(fmt.Sprintf("persona %s failed; fixed fallback/skip: %v", j.event.Scene, err))
		out = j.event.Fallback
	}
	if strings.TrimSpace(out) == "[SKIP]" {
		return
	}
	if j.ticket.Idle && len(r.high) > 0 {
		return
	}
	out = Fit(out, j.event.Name, r.cfg.MaxUnits)
	if out == "" {
		return
	}
	r.mu.Lock()
	dup := false
	for _, s := range r.recent {
		if s == out {
			dup = true
			break
		}
	}
	r.mu.Unlock()
	if j.ticket.Idle && dup {
		r.log("skip duplicate idle text")
		return
	}
	j.send(out, func() bool {
		now := time.Now()
		r.mu.Lock()
		defer r.mu.Unlock()
		if !r.valid(j.ticket, now) {
			return false
		}

		return true
	})
}
