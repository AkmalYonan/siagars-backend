package openrouter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"siagars/backend/internal/config"
	"siagars/backend/internal/service"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSelectUsesPinnedFreeProviderAndStrictSchema(t *testing.T) {
	c := New(config.AssistantLLMConfig{APIKey: "test-key", Model: "test/model:free", Provider: "test/endpoint"})
	c.http.Transport = transportFunc(func(req *http.Request) (*http.Response, error) {
		var body struct {
			Model    string
			Messages []struct{ Role, Content string }
			Provider struct {
				Only              []string
				RequireParameters bool   `json:"require_parameters"`
				AllowFallbacks    *bool  `json:"allow_fallbacks"`
				DataCollection    string `json:"data_collection"`
				ZDR               bool
			}
			ResponseFormat struct {
				Type       string
				JSONSchema struct{ Strict bool } `json:"json_schema"`
			} `json:"response_format"`
		}
		if req.URL.String() != endpoint || req.Header.Get("Authorization") != "Bearer test-key" ||
			json.NewDecoder(req.Body).Decode(&body) != nil {
			t.Fatal("invalid outbound request")
		}
		if body.Model != "test/model:free" || len(body.Provider.Only) != 1 || body.Provider.Only[0] != "test/endpoint" ||
			!body.Provider.RequireParameters || body.Provider.AllowFallbacks == nil || *body.Provider.AllowFallbacks ||
			body.Provider.DataCollection != "deny" || !body.Provider.ZDR || body.ResponseFormat.Type != "json_schema" ||
			!body.ResponseFormat.JSONSchema.Strict || len(body.Messages) != 2 || body.Messages[1].Role != "user" ||
			body.Messages[1].Content != `{"terms":["icu","nggak perlu","nicu"],"service_ids":["icu","nicu"]}` {
			t.Fatalf("unexpected provider config: %+v", body.Provider)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(
			`{"choices":[{"finish_reason":"stop","message":{"content":"{\"action\":\"select\",\"service_ids\":[\"nicu\"]}"}}]}`,
		))}, nil
	})
	got, err := c.Select(context.Background(), service.AssistantHints{Terms: []string{"icu", "nggak perlu", "nicu"}, ServiceIDs: []string{"icu", "nicu"}})
	if err != nil || got.Action != "select" || len(got.ServiceIDs) != 1 || got.ServiceIDs[0] != "nicu" {
		t.Fatalf("selection=%+v err=%v", got, err)
	}
}

func TestSelectRejectsUpstreamErrorsAndUnexpectedContent(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
	}{
		{429, "upstream payload"},
		{200, "invalid JSON"},
		{200, `{"choices":[{"finish_reason":"length","message":{"content":"{}"}}]}`},
		{200, `{"choices":[{"finish_reason":"stop","message":{"content":"{\"action\":\"clarify\",\"service_ids\":[],\"reply\":\"unsafe\"}"}}]}`},
		{200, `{"choices":[{"finish_reason":"stop","message":{"content":"{\"action\":\"clarify\",\"service_ids\":null}"}}]}`},
	} {
		c := New(config.AssistantLLMConfig{})
		c.http.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
		})
		if _, err := c.Select(context.Background(), service.AssistantHints{ServiceIDs: []string{"nicu"}}); err == nil || strings.Contains(err.Error(), "upstream payload") {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}
