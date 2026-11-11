package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"siagars/backend/internal/integrations/openrouter"
	"siagars/backend/internal/service"
	"strings"
	"testing"
)

type mockAssistantAnswerer struct {
	answer func(string, map[string]string) ([]string, error)
}

func (m mockAssistantAnswerer) Answer(_ context.Context, question string, passages map[string]string) ([]string, error) {
	return m.answer(question, passages)
}

func (mockAssistantAnswerer) Select(context.Context, service.AssistantHints) (openrouter.Selection, error) {
	panic("unexpected filter extraction")
}

func TestAssistantConversationUsesGroundedFallback(t *testing.T) {
	for _, tc := range []struct {
		question, want, model string
		ids                   []string
		call                  bool
	}{
		{"Halo", "Halo! Saya asisten SiagaRS", "openrouter-answer-v1", []string{"greeting"}, true},
		{"Bisa lihat perkembangan pengajuan dari mana?", "/lacak-rujukan", "openrouter-answer-v1", []string{"referral"}, true},
		{"Jelaskan fisika kuantum", "bisa membantu mencari", "openrouter-answer-v1", []string{"outside"}, true},
		{"Apa yang bisa kamu bantu?", "bisa membantu mencari", "rule-nlp-v1", []string{"unknown"}, true},
		{"Saya Nadia, NIK 1234567890123456", "bisa membantu mencari", "rule-nlp-v1", nil, false},
		{"Pasien tidak sadar", "119", "rule-nlp-v1", nil, false},
		{"Cara cek BPJS?", "/cek-bpjs", "rule-nlp-v1", nil, false},
	} {
		t.Run(tc.question, func(t *testing.T) {
			called := false
			s := &Server{
				repo: assistantTestStore{}, sharedLimiter: &assistantQuota{},
				assistantLLM: mockAssistantAnswerer{answer: func(question string, passages map[string]string) ([]string, error) {
					called = true
					if question != tc.question || passages["referral"] == "" {
						t.Fatalf("unexpected model input")
					}
					return tc.ids, nil
				}},
			}
			body, _ := json.Marshal(map[string]string{"message": tc.question})
			w := assistantRequest(s.middleware(http.HandlerFunc(s.assistantChat)), string(body))
			var res struct {
				Data assistantChatData
				Meta struct{ Model string }
			}
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &res) != nil || called != tc.call ||
				res.Meta.Model != tc.model || !strings.Contains(res.Data.Reply, tc.want) {
				t.Fatalf("called=%v: HTTP %d %s", called, w.Code, w.Body.String())
			}
		})
	}
}
