package httpapi

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"siagars/backend/internal/config"
	"siagars/backend/internal/domain"
	"strings"
	"testing"
	"time"
)

const jobSecretValue = "job-secret-for-tests-0123456789abcdef"

type jobStore struct {
	Store
	runs  []string
	fail  bool
	start time.Time
}

func (j *jobStore) RefreshSimulationJob(_ context.Context, actor, rid string, at time.Time) (domain.SimulationRefresh, error) {
	if j.fail {
		return domain.SimulationRefresh{}, errors.New("db down")
	}
	j.runs = append(j.runs, actor+"|"+rid)
	return domain.SimulationRefresh{At: at, ImportedHospitals: 516, SimulationHospitals: 30, Services: 3276, Beds: 1700}, nil
}

func jobCall(h http.Handler, method, query string, header map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/api/v1/internal/jobs/refresh-simulation"+query, nil)
	for k, v := range header {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestRefreshSimulationJobAuth(t *testing.T) {
	store := &jobStore{}
	h := New(store, config.Config{JobSecret: jobSecretValue}, func(context.Context) error { return nil })
	for name, c := range map[string]struct {
		method string
		header map[string]string
	}{
		"tanpa secret":           {"POST", nil},
		"secret salah":           {"POST", map[string]string{"X-Job-Secret": jobSecretValue + "x"}},
		"secret pendek":          {"POST", map[string]string{"X-Job-Secret": "job"}},
		"bearer di POST":         {"POST", map[string]string{"Authorization": "Bearer " + jobSecretValue}},
		"header job di GET":      {"GET", map[string]string{"X-Job-Secret": jobSecretValue}},
		"GET tanpa Bearer":       {"GET", map[string]string{"Authorization": jobSecretValue}},
		"GET bearer salah":       {"GET", map[string]string{"Authorization": "Bearer salah"}},
		"secret di query string": {"POST", nil},
	} {
		query := ""
		if name == "secret di query string" {
			query = "?secret=" + jobSecretValue
		}
		if w := jobCall(h, c.method, query, c.header); w.Code != 401 || errCode(w) != "unauthorized" {
			t.Errorf("%s: %d %s", name, w.Code, w.Body.String())
		}
	}
	if len(store.runs) != 0 {
		t.Fatal("job berjalan tanpa secret yang sah:", store.runs)
	}

	w := jobCall(h, "POST", "", map[string]string{"X-Job-Secret": jobSecretValue})
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Body.String(), `"simulation_hospitals":30`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = jobCall(h, "GET", "", map[string]string{"Authorization": "Bearer " + jobSecretValue})
	if w.Code != 200 || len(store.runs) != 2 || !strings.HasPrefix(store.runs[0], "system:cron|") {
		t.Fatal(w.Code, store.runs)
	}
	if w := jobCall(h, "POST", "?x=1", map[string]string{"X-Job-Secret": jobSecretValue}); w.Code != 422 {
		t.Fatal("parameter asing:", w.Code)
	}
	if w := jobCall(h, "PUT", "", map[string]string{"X-Job-Secret": jobSecretValue}); w.Code != 405 || w.Header().Get("Allow") != "POST, GET" {
		t.Fatal("PUT:", w.Code, w.Header().Get("Allow"))
	}
}

func TestRefreshSimulationJobDisabledWithoutSecretAndNeverLogsIt(t *testing.T) {
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(old)
	store := &jobStore{}
	// JOB_SECRET unset: even an empty header must not match the empty configured secret.
	off := New(store, config.Config{}, func(context.Context) error { return nil })
	for _, hdr := range []map[string]string{nil, {"X-Job-Secret": ""}} {
		if w := jobCall(off, "POST", "", hdr); w.Code != 401 {
			t.Fatal("tanpa JOB_SECRET:", w.Code)
		}
	}
	on := New(store, config.Config{JobSecret: jobSecretValue}, func(context.Context) error { return nil })
	jobCall(on, "POST", "", map[string]string{"X-Job-Secret": jobSecretValue})
	store.fail = true
	if w := jobCall(on, "POST", "", map[string]string{"X-Job-Secret": jobSecretValue}); w.Code != 500 {
		t.Fatal("kegagalan job:", w.Code)
	}
	if strings.Contains(logs.String(), jobSecretValue) {
		t.Fatal("secret job tercatat di log")
	}
	unsupported := New(&secStore{}, config.Config{JobSecret: jobSecretValue}, func(context.Context) error { return nil })
	if w := jobCall(unsupported, "POST", "", map[string]string{"X-Job-Secret": jobSecretValue}); w.Code != 501 {
		t.Fatal("store tanpa job:", w.Code)
	}
}
