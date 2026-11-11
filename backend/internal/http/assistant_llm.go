package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"siagars/backend/internal/integrations/openrouter"
	"siagars/backend/internal/service"
	"slices"
	"time"
)

type assistantSelector interface {
	Select(context.Context, service.AssistantHints) (openrouter.Selection, error)
}

// interpretAssistantServices returns an empty list when local clarification is needed.
func (s *Server) interpretAssistantServices(r *http.Request, hints service.AssistantHints) []string {
	if s.assistantLLM == nil || len(hints.Terms) == 0 {
		return nil
	}
	started := time.Now()
	outcome := "quota_unavailable"
	defer func() {
		slog.Info("assistant llm", "request_id", rid(r), "outcome", outcome, "duration_ms", time.Since(started).Milliseconds())
	}()
	if s.sharedLimiter == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	quotaCtx, stopQuota := context.WithTimeout(ctx, 500*time.Millisecond)
	defer stopQuota()
	// Charge the minute window first so bursts cannot exhaust the daily allocation.
	for _, limit := range []struct {
		key    string
		max    int
		window time.Duration
	}{
		{"assistant-llm-minute", 10, time.Minute},
		{"assistant-llm-day", 40, 24 * time.Hour},
	} {
		allowed, err := s.sharedLimiter.TakeRateLimit(quotaCtx, limit.key, limit.max, limit.window, started)
		if err != nil {
			return nil
		}
		if !allowed {
			outcome = "quota_exhausted"
			return nil
		}
	}
	stopQuota()
	selection, err := s.assistantLLM.Select(ctx, hints)
	if err != nil {
		// The adapter returns fixed error codes without upstream response bodies.
		outcome = err.Error()
		return nil
	}
	if selection.Action == "clarify" && len(selection.ServiceIDs) == 0 {
		outcome = "clarification"
		return nil
	}
	outcome = "invalid_selection"
	if selection.Action != "select" || len(selection.ServiceIDs) == 0 || len(selection.ServiceIDs) >= len(hints.ServiceIDs) {
		return nil
	}
	ids := slices.Clone(selection.ServiceIDs)
	slices.Sort(ids)
	if len(slices.Compact(slices.Clone(ids))) != len(ids) {
		return nil
	}
	for _, id := range ids {
		if !slices.Contains(hints.ServiceIDs, id) || slices.Contains(hints.ExcludedServiceIDs, id) {
			return nil
		}
	}
	outcome = "selected"
	return ids
}
