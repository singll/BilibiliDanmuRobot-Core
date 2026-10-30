package danmu

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var blindBoxLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return loc
}()

var blindBoxMonthPattern = regexp.MustCompile(`^([0-9零〇一二两三四五六七八九十百]+)月$`)

type blindBoxPeriod struct {
	label      string
	start, end time.Time // [start, end); both zero means all history
}

func parseBlindBoxPeriod(msg string, now time.Time) (blindBoxPeriod, bool, error) {
	msg = strings.TrimSpace(msg)
	if msg == "盲盒统计" {
		return blindBoxPeriod{label: "累计"}, true, nil
	}
	msg = strings.TrimSuffix(msg, "统计")
	if !strings.HasSuffix(msg, "盲盒") {
		return blindBoxPeriod{}, false, nil
	}
	prefix := strings.TrimSuffix(msg, "盲盒")
	now = now.In(blindBoxLocation)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, blindBoxLocation)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, blindBoxLocation)
	weekStart := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
	var p blindBoxPeriod
	switch prefix {
	case "今日", "今天", "当日", "当天":
		p = blindBoxPeriod{"今日", today, today.AddDate(0, 0, 1)}
	case "昨日", "昨天":
		p = blindBoxPeriod{"昨日", today.AddDate(0, 0, -1), today}
	case "前日", "前天":
		p = blindBoxPeriod{"前日", today.AddDate(0, 0, -2), today.AddDate(0, 0, -1)}
	case "本周", "这周", "本星期", "这个星期":
		p = blindBoxPeriod{"本周", weekStart, weekStart.AddDate(0, 0, 7)}
	case "上周", "上星期", "上个星期":
		p = blindBoxPeriod{"上周", weekStart.AddDate(0, 0, -7), weekStart}
	case "", "本月", "这个月", "当月":
		p = blindBoxPeriod{"本月", monthStart, monthStart.AddDate(0, 1, 0)}
	case "上月", "上个月":
		p = blindBoxPeriod{"上月", monthStart.AddDate(0, -1, 0), monthStart}
	default:
		match := blindBoxMonthPattern.FindStringSubmatch(prefix)
		if match == nil {
			return blindBoxPeriod{}, false, nil
		}
		month, err := strconv.Atoi(match[1])
		if err != nil {
			month = map[string]int{"一": 1, "二": 2, "两": 2, "三": 3, "四": 4, "五": 5, "六": 6,
				"七": 7, "八": 8, "九": 9, "十": 10, "十一": 11, "十二": 12}[match[1]]
		}
		if month < 1 || month > 12 {
			return blindBoxPeriod{}, true, fmt.Errorf("月份「%s」不正确!", match[1])
		}
		start := time.Date(now.Year(), time.Month(month), 1, 0, 0, 0, 0, blindBoxLocation)
		p = blindBoxPeriod{fmt.Sprintf("%d月", month), start, start.AddDate(0, 1, 0)}
	}
	return p, true, nil
}
