package companion

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFullDisplayBudget(t *testing.T) {
	for _, name := range []string{"", "鱼清悦", "Alice", "鱼🐟清悦", strings.Repeat("悦", 38)} {
		out := Fit(strings.Repeat("很喜欢清悦💙", 20), name, 80)
		prefix := ""
		if name != "" {
			prefix = "@" + name + " "
		}
		if Units(prefix+out) > 80 {
			t.Fatalf("over budget: %q", prefix+out)
		}
		if !utf8.ValidString(out) {
			t.Fatal("invalid UTF8")
		}
	}
	if out := Fit("你好", "小鱼", 80); out != "你好" {
		t.Fatal(out)
	}
	if out := Fit("你好", strings.Repeat("鱼", 40), 80); out != "" {
		t.Fatal(out)
	}
}
func TestGraphemeClipping(t *testing.T) {
	for _, emoji := range []string{"💙", "👩‍👩‍👧‍👦", "👍🏽", "🇨🇳", "❤️"} {
		if out := Clip("好"+emoji+"呀", 2+Units(emoji)-1); out != "好" {
			t.Fatalf("split emoji %q: %q", emoji, out)
		}
	}
	if Units(strings.Repeat("鱼", 40)) != 80 {
		t.Fatal("CJK budget")
	}
}
func TestClean(t *testing.T) {
	got := Clean("**好呢**\n[链接](https://example.com) https://x.com/a\n鱼清悦💙")
	if got != "好呢 链接 鱼清悦💙" {
		t.Fatal(got)
	}
	if strings.Contains(Clean("请看www.example.com/1呀"), "www.") {
		t.Fatal("URL remained")
	}
}
