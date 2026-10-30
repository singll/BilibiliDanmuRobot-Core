package danmu

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/model"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/svc"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestBlindBoxPeriodAliases(t *testing.T) {
	now := time.Date(2026, 10, 30, 22, 0, 0, 0, blindBoxLocation)
	for _, tc := range []struct {
		commands          string
		label, start, end string
	}{
		{"今日盲盒 今天盲盒 当日盲盒 当天盲盒 今日盲盒统计", "今日", "2026-10-30", "2026-10-31"},
		{"昨日盲盒 昨天盲盒", "昨日", "2026-10-29", "2026-10-30"},
		{"前日盲盒 前天盲盒", "前日", "2026-10-28", "2026-10-29"},
		{"本周盲盒 这周盲盒 本星期盲盒 这个星期盲盒 本周盲盒统计", "本周", "2026-10-26", "2026-11-02"},
		{"上周盲盒 上星期盲盒 上个星期盲盒", "上周", "2026-10-19", "2026-10-26"},
		{"本月盲盒 这个月盲盒 当月盲盒 盲盒 本月盲盒统计", "本月", "2026-10-01", "2026-11-01"},
		{"上月盲盒 上个月盲盒", "上月", "2026-09-01", "2026-10-01"},
		{"10月盲盒 十月盲盒 010月盲盒 十月盲盒统计", "10月", "2026-10-01", "2026-11-01"},
	} {
		for _, command := range strings.Fields(tc.commands) {
			t.Run(command, func(t *testing.T) {
				p, matched, err := parseBlindBoxPeriod(" \t"+command+"\n", now)
				if err != nil || !matched || p.label != tc.label || p.start.Format("2006-01-02") != tc.start || p.end.Format("2006-01-02") != tc.end {
					t.Fatalf("period=%+v matched=%v err=%v", p, matched, err)
				}
			})
		}
	}
	for i, cn := range []string{"一", "二", "三", "四", "五", "六", "七", "八", "九", "十", "十一", "十二"} {
		for _, month := range []string{cn, fmt.Sprint(i + 1)} {
			p, matched, err := parseBlindBoxPeriod(month+"月盲盒", now)
			if err != nil || !matched || p.start.Year() != 2026 || p.start.Month() != time.Month(i+1) || p.start.Day() != 1 || p.end != p.start.AddDate(0, 1, 0) {
				t.Fatalf("month %s: %+v %v %v", month, p, matched, err)
			}
		}
	}
	p, matched, err := parseBlindBoxPeriod("盲盒统计", now)
	if err != nil || !matched || p.label != "累计" || !p.start.IsZero() || !p.end.IsZero() {
		t.Fatalf("all history: %+v %v %v", p, matched, err)
	}
}

func TestBlindBoxPeriodBoundaries(t *testing.T) {
	for _, tc := range []struct{ now, command, start, end string }{
		{"2026-10-31T15:59:59Z", "今日盲盒", "2026-10-31", "2026-11-01"},
		{"2026-10-31T16:00:00Z", "今日盲盒", "2026-11-01", "2026-11-02"},
		{"2026-11-01T15:59:59Z", "本周盲盒", "2026-10-26", "2026-11-02"},
		{"2026-11-01T16:00:00Z", "本周盲盒", "2026-11-02", "2026-11-09"},
		{"2026-01-01T00:00:00Z", "本周盲盒", "2025-12-29", "2026-01-05"},
		{"2026-01-01T00:00:00Z", "上月盲盒", "2025-12-01", "2026-01-01"},
		{"2026-01-01T00:00:00Z", "昨天盲盒", "2025-12-31", "2026-01-01"},
		{"2024-02-29T00:00:00Z", "本月盲盒", "2024-02-01", "2024-03-01"},
		{"2024-03-01T00:00:00Z", "昨天盲盒", "2024-02-29", "2024-03-01"},
	} {
		t.Run(tc.now+tc.command, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, tc.now)
			if err != nil {
				t.Fatal(err)
			}
			p, matched, err := parseBlindBoxPeriod(tc.command, now)
			if err != nil || !matched || p.start.Format("2006-01-02") != tc.start || p.end.Format("2006-01-02") != tc.end || p.start.Hour() != 0 || p.end.Hour() != 0 {
				t.Fatalf("period=%+v matched=%v err=%v", p, matched, err)
			}
		})
	}
}

func TestBlindBoxPeriodRejectsInvalidCommands(t *testing.T) {
	for _, msg := range []string{"0月盲盒", "13月盲盒", "零月盲盒", "十三月盲盒", "一百月盲盒", "999999999999999999999月盲盒"} {
		if _, matched, err := parseBlindBoxPeriod(msg, time.Now()); !matched || err == nil {
			t.Errorf("%q: matched=%v err=%v", msg, matched, err)
		}
	}
	for _, msg := range []string{"", "我今天盲盒赚了", "今日盲盒怎么样", "今天", "月份盲盒", "盲盒统计统计", "盲盒好贵", "a10月盲盒", "-1月盲盒", "1.5月盲盒"} {
		if _, matched, err := parseBlindBoxPeriod(msg, time.Now()); matched || err != nil {
			t.Errorf("%q: matched=%v err=%v", msg, matched, err)
		}
	}
}

func TestBlindBoxQueriesOnExistingDateSchema(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "blind.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	stats := model.NewBlindBoxStatModel(db, 123)
	for _, row := range []struct {
		date         string
		uid          int64
		count, price int32
	}{
		{"2025-12-28", 101, 1, 2000},
		{"2025-12-29", 101, 2, 1500},
		{"2025-12-31", 101, 3, 500},
		{"2026-01-01", 101, 4, 1250},
		{"2026-01-04", 101, 5, 1000},
		{"2026-01-05", 101, 6, 1100},
		{"2025-01-01", 101, 7, 1000},
		{"2026-01-01", 202, 8, 500},
	} {
		date, err := time.Parse("2006-01-02", row.date)
		if err != nil {
			t.Fatal(err)
		}
		if err := stats.Insert(context.Background(), nil, &model.BlindBoxStatBase{
			Uid: row.uid, Cnt: row.count, Price: row.price, OriginalGiftPrice: 1000,
			Year: int16(date.Year()), Month: int16(date.Month()), Day: int16(date.Day()),
		}); err != nil {
			t.Fatal(err)
		}
	}
	otherRoom := model.NewBlindBoxStatModel(db, 456)
	if err := otherRoom.Insert(context.Background(), nil, &model.BlindBoxStatBase{Uid: 101, Cnt: 999, Price: 999999, Year: 2026, Month: 1, Day: 1}); err != nil {
		t.Fatal(err)
	}
	svcCtx := &svc.ServiceContext{UserID: 999, BlindBoxStatModel: stats}
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, blindBoxLocation)
	for _, tc := range []struct{ msg, uid, want string }{
		{"今日盲盒", "101", "今日数量4个，盈亏+1.00元"},
		{"昨天盲盒", "101", "昨日数量3个，盈亏-1.50元"},
		{"本周盲盒", "101", "本周数量14个，盈亏+0.50元"},
		{"上周盲盒", "101", "上周数量1个，盈亏+1.00元"},
		{"本月盲盒", "101", "本月数量15个，盈亏+1.60元"},
		{"一月盲盒", "101", "1月数量15个，盈亏+1.60元"},
		{"上月盲盒", "101", "上月数量6个，盈亏+0.50元"},
		{"盲盒统计", "101", "累计数量28个，盈亏+2.10元"},
		{"盲盒统计", "202", "累计数量8个，盈亏-4.00元"},
		{"盲盒统计", "999", "累计数量36个，盈亏-1.90元"},
		{"今天盲盒", "999", "今日数量12个，盈亏-3.00元"},
		{"本周盲盒", "999", "本周数量22个，盈亏-3.50元"},
		{"本月盲盒", "999", "本月数量23个，盈亏-2.40元"},
		{"盲盒统计", "303", "累计暂无相关记录"},
		{"二月盲盒", "999", "2月暂无相关记录"},
		{"13月盲盒", "101", "月份「13」不正确!"},
		{"今天盲盒真好玩", "101", ""},
	} {
		t.Run(tc.msg+tc.uid, func(t *testing.T) {
			got, err := queryBlindBoxStat(context.Background(), tc.msg, tc.uid, svcCtx, now)
			if err != nil || got != tc.want {
				t.Fatalf("got %q err=%v; want %q", got, err, tc.want)
			}
		})
	}
	breakEvenDate := time.Date(2026, 1, 4, 12, 0, 0, 0, blindBoxLocation)
	if got, err := queryBlindBoxStat(context.Background(), "今天盲盒", "101", svcCtx, breakEvenDate); err != nil || got != "今日数量5个，盈亏+0.00元" {
		t.Fatalf("break even: %q %v", got, err)
	}
	// UID parsing must stop before a database call; an unknown broadcaster (0)
	// must never turn an invalid viewer UID into a room-wide query.
	for _, uid := range []string{"", "bad", "0", "-1", "9223372036854775808"} {
		if _, err := queryBlindBoxStat(context.Background(), "盲盒统计", uid, &svc.ServiceContext{}, now); err == nil {
			t.Errorf("accepted uid %q", uid)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := queryBlindBoxStat(ctx, "今日盲盒", "101", svcCtx, now); err == nil {
		t.Error("ignored database/context error")
	}
}
