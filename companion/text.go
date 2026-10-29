package companion

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
)

// Units implements the configured display budget: ASCII=1, BMP=2,
// supplementary characters (including most emoji)=4. This is deliberately
// conservative; truncation never splits a grapheme such as a joined emoji.
func Units(s string) int {
	n := 0
	for _, r := range s {
		if r < 128 {
			n++
		} else if r <= 0xffff {
			n += 2
		} else {
			n += 4
		}
	}
	return n
}

func Clip(s string, budget int) string {
	if budget <= 0 {
		return ""
	}
	g := uniseg.NewGraphemes(s)
	var b strings.Builder
	for g.Next() {
		part := g.Str()
		cost := Units(part)
		if cost > budget {
			break
		}
		b.WriteString(part)
		budget -= cost
	}
	return b.String()
}

var markdownLink = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
var urlPattern = regexp.MustCompile(`(?i)(?:https?://|www\.)[^\s]+|(?:[a-z0-9-]+\.)+(?:com|net|org|cn|io|tv|gg|me|app|dev|co|xyz)(?:/[^\s]*)?`)

func Clean(s string) string {
	s = markdownLink.ReplaceAllString(s, "$1")
	s = urlPattern.ReplaceAllString(s, "")
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return ' '
		}
		if strings.ContainsRune("`*_#>|", r) {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

func ReplyBudget(limit int, name string) int {
	if limit <= 0 || limit > 80 {
		limit = 80
	}
	if name != "" {
		return limit - Units("@"+name+" ")
	}
	return limit
}

// Fit checks the full displayed reply, including the native Bilibili mention.
func Fit(s, name string, limit int) string {
	return strings.TrimSpace(Clip(Clean(s), ReplyBudget(limit, name)))
}
