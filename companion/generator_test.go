package companion

import (
	"context"
	"encoding/json"
	"github.com/Hello-Moeka/BilibiliDanmuRobot-Core/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGenerateUsesPersonaAndConfiguredPool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/llm/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("wrong endpoint/auth")
		}
		var body struct {
			Model     string `json:"model"`
			Messages  []struct{ Role, Content string }
			Thinking  struct{ Type string }
			MaxTokens int `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if !strings.Contains(body.Messages[0].Content, "鱼清幻") {
			t.Error("missing shared persona")
		}
		if body.Model != "pool-bili-deepseek-paid" || len(body.Messages) != 2 || body.Messages[0].Role != "system" || body.Thinking.Type != "disabled" || body.MaxTokens != 256 {
			t.Errorf("unexpected request: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"好呢💙"}}]}`))
	}))
	defer server.Close()
	for _, scene := range []string{"chat", "welcome", "gift", "clock", "idle"} {
		out, err := Generate(context.Background(), config.ChatGPTConfig{APIUrl: server.URL + "/api/llm/v1", APIToken: "test-key", Prompt: "鱼清幻", Model: "pool-bili-deepseek-paid"}, 256, Event{Scene: scene, Facts: "观众来了", Budget: 70})
		if err != nil || out != "好呢💙" {
			t.Fatal(out, err)
		}
	}
}
func TestGenerateHonorsCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := Generate(ctx, config.ChatGPTConfig{APIUrl: server.URL}, 32, Event{})
	if err == nil {
		t.Fatal("expected timeout")
	}
}
