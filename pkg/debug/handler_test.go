package debug

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"maps"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"reviewsrv/pkg/reviewer"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
)

func newTestHandler(t *testing.T) (*Storage, *echo.Echo) {
	t.Helper()
	return newTestHandlerWith(t, nil, nil)
}

// newTestHandlerWith builds the handler with a project lookup and run metrics.
func newTestHandlerWith(t *testing.T, lookup func(context.Context, string) (string, error), metrics *reviewer.RunMetrics) (*Storage, *echo.Echo) {
	t.Helper()
	storage := New(5, 5)
	h := NewHandler(storage, slog.Default(), lookup, metrics)

	e := echo.New()
	e.POST("/v1/upload/debug/:projectKey/", h.Upload)
	e.GET("/v1/debug/storage/", h.List)
	e.GET("/v1/debug/storage/:id/", h.Bundle)
	e.GET("/v1/debug/storage/:id/:filename", h.File)
	return storage, e
}

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(data); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

// gzFile builds a stored artifact from its content.
func gzFile(t *testing.T, content string) File {
	t.Helper()
	return File{Gzip: gzipBytes(t, []byte(content)), Size: len(content)}
}

// unzip returns a stored artifact's content.
func unzip(t *testing.T, f File) string {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(f.Gzip))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	data, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	return string(data)
}

func buildMultipart(t *testing.T, fields map[string]string, files map[string][]byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	for name, content := range files {
		w, err := mw.CreateFormFile("file", name+".gz")
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := w.Write(gzipBytes(t, content)); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return &buf, mw.FormDataContentType()
}

func TestHandler_UploadStoresBundle(t *testing.T) {
	storage, e := newTestHandler(t)

	projectKey := uuid.NewString()
	body, ct := buildMultipart(t,
		map[string]string{
			"mrIid":        "42",
			"externalId":   "ext-7",
			"runner":       "claude",
			"model":        "opus",
			"errorMsg":     "validate review.json: invalid reviewType: ",
			"sourceBranch": "feat/x",
			"targetBranch": "master",
			"commitHash":   "abc123",
		},
		map[string][]byte{
			"review.json":        []byte(`{"files":[]}`),
			"claude-output.json": []byte(`{"type":"result"}`),
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/v1/upload/debug/"+projectKey+"/", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["id"] == "" {
		t.Fatal("response id is empty")
	}

	b := storage.Get(resp["id"])
	if b == nil {
		t.Fatal("bundle not stored")
	}
	if b.MRIid != "42" || b.Runner != "claude" || b.Model != "opus" {
		t.Errorf("metadata mismatch: %+v", b)
	}
	if got := unzip(t, b.Files["review.json"]); got != `{"files":[]}` {
		t.Errorf("review.json content mismatch: %q", got)
	}
	if f := b.Files["claude-output.json"]; unzip(t, f) != `{"type":"result"}` || f.Size != len(`{"type":"result"}`) {
		t.Errorf("claude-output.json mismatch: %q (size %d)", unzip(t, f), f.Size)
	}
}

func TestHandler_UploadRejectsInvalidProjectKey(t *testing.T) {
	_, e := newTestHandler(t)
	body, ct := buildMultipart(t, nil, map[string][]byte{"x": []byte("y")})

	req := httptest.NewRequest(http.MethodPost, "/v1/upload/debug/not-a-uuid/", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandler_UploadRejectsMalformedGzip(t *testing.T) {
	_, e := newTestHandler(t)
	projectKey := uuid.NewString()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	w, _ := mw.CreateFormFile("file", "broken.json.gz")
	_, _ = w.Write([]byte("not gzip"))
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/upload/debug/"+projectKey+"/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandler_ListAndBundleHTML(t *testing.T) {
	storage, e := newTestHandler(t)
	storage.Add(&Bundle{
		ID:           "abc123",
		ProjectKey:   "11111111-2222-3333-4444-555555555555",
		MRIid:        "99",
		Runner:       "claude",
		Model:        "opus",
		ErrorMsg:     "boom",
		SourceBranch: "feat/x",
		TargetBranch: "master",
		Files:        map[string]File{"review.json": gzFile(t, `{"x":1}`)},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/debug/storage/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "abc123") {
		t.Error("list HTML missing bundle id")
	}
	if !strings.Contains(rec.Body.String(), "boom") {
		t.Error("list HTML missing error preview")
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/debug/storage/abc123/", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("bundle status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "review.json") {
		t.Error("bundle HTML missing file row")
	}
	if !strings.Contains(body, "claude") {
		t.Error("bundle HTML missing runner")
	}
}

func TestHandler_FileServesArtifactWithContentType(t *testing.T) {
	storage, e := newTestHandler(t)
	storage.Add(&Bundle{
		ID:    "xyz",
		Files: map[string]File{"review.json": gzFile(t, `{"ok":true}`)},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/debug/storage/xyz/review.json", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q, want application/json prefix", got)
	}
	body, _ := io.ReadAll(rec.Body)
	if string(body) != `{"ok":true}` {
		t.Errorf("body = %q", body)
	}
}

func TestHandler_FileNeverServesARenderableType(t *testing.T) {
	storage, e := newTestHandler(t)
	storage.Add(&Bundle{ID: "xss", Files: map[string]File{
		"x.html": gzFile(t, "<script>alert(1)</script>"),
		"x.svg":  gzFile(t, `<svg onload="alert(1)"/>`),
	}})

	for _, name := range []string{"x.html", "x.svg"} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/debug/storage/xss/"+name, nil))
		if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
			t.Errorf("%s: Content-Type = %q, want text/plain", name, got)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Content-Security-Policy") != "sandbox" {
			t.Errorf("%s: missing nosniff/sandbox headers: %v", name, rec.Header())
		}
	}
}

func TestHandler_UploadRefusedWhenProjectLookupFails(t *testing.T) {
	_, e := newTestHandlerWith(t, func(context.Context, string) (string, error) {
		return "", errors.New("db down")
	}, nil)

	body, ct := buildMultipart(t, map[string]string{"errorMsg": "boom"}, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/upload/debug/"+uuid.NewString()+"/", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503: an unverified key is not accepted", rec.Code)
	}
}

func TestHandler_UploadHidesProjectKey(t *testing.T) {
	storage, e := newTestHandler(t)
	key := uuid.NewString()
	b := upload(t, storage, e, key, map[string]string{
		"errorMsg": `upload: Post "https://reviewer/v1/reviewctl/upload/` + key + `/": EOF`,
		"reviewId": "+42",
	})
	if strings.Contains(b.ErrorMsg, key) {
		t.Errorf("errorMsg reveals the project key: %q", b.ErrorMsg)
	}
	if b.ReviewID != "42" {
		t.Errorf("reviewId = %q, want the canonical 42", b.ReviewID)
	}

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/debug/storage/"+b.ID+"/", nil))
	if strings.Contains(rec.Body.String(), key) {
		t.Error("the bundle page reveals the project key")
	}
}

func TestHandler_FileNotFound(t *testing.T) {
	_, e := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/debug/storage/missing/review.json", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// upload posts a bundle and returns the stored copy.
func upload(t *testing.T, storage *Storage, e *echo.Echo, projectKey string, fields map[string]string) *Bundle {
	t.Helper()
	body, ct := buildMultipart(t, fields, map[string][]byte{"review.json": []byte(`{}`)})
	req := httptest.NewRequest(http.MethodPost, "/v1/upload/debug/"+projectKey+"/", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return storage.Get(resp["id"])
}

func TestHandler_UploadRecordsRunOutcome(t *testing.T) {
	knownKey := uuid.NewString()
	metrics := reviewer.NewRunMetrics("claude")
	reg := prometheus.NewRegistry()
	metrics.Register(reg)
	storage, e := newTestHandlerWith(t, func(_ context.Context, key string) (string, error) {
		if key != knownKey {
			return "", nil // no such project
		}
		return "demo", nil
	}, metrics)

	b := upload(t, storage, e, knownKey, map[string]string{
		"runner": "claude", "errorMsg": "run claude: billing_error: Credit balance is too low",
		"status": "failed", "reason": "billing", "costUsd": "1.5",
	})
	if b.Status != "failed" || b.Reason != "billing" || b.CostUsd != 1.5 || b.ProjectTitle != "demo" {
		t.Errorf("outcome not recorded: %+v", b)
	}

	// A legacy client sends no status: derived from errorMsg. Garbage is
	// sanitized; an implausible cost stays out of the cost counter.
	b = upload(t, storage, e, knownKey, map[string]string{"errorMsg": "boom", "reason": "<script>", "costUsd": "5000"})
	if b.Status != "failed" || b.Reason != "other" {
		t.Errorf("legacy upload not normalized: %+v", b)
	}
	// An ok run is counted on review upload, not here — and so is a run whose
	// review was created before it failed.
	upload(t, storage, e, knownKey, map[string]string{"runner": "claude", "status": "ok", "costUsd": "2"})
	b = upload(t, storage, e, knownKey, map[string]string{"runner": "claude", "status": "failed", "reason": "other", "costUsd": "3", "reviewId": "80"})
	if b.ReviewID != "80" {
		t.Errorf("reviewId not stored: %+v", b)
	}
	// A garbage reviewId can't switch the metrics off: this one is counted.
	upload(t, storage, e, knownKey, map[string]string{"runner": "claude", "status": "failed", "reason": "billing", "reviewId": "x"})

	// A key that matches no project is refused.
	body, ct := buildMultipart(t, map[string]string{"errorMsg": "x", "costUsd": "999"}, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/upload/debug/"+uuid.NewString()+"/", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown project: status %d, want 404", rec.Code)
	}

	type labels = map[string]string
	for _, c := range []struct {
		metric string
		labels labels
		want   float64
	}{
		{"reviewer_runs_total", labels{"project": "demo", "runner": "claude", "status": "failed", "reason": "billing"}, 2},
		{"reviewer_runs_total", labels{"project": "demo", "runner": "other", "status": "failed", "reason": "other"}, 1},
		{"reviewer_run_cost_usd_total", labels{"project": "demo", "runner": "claude", "status": "failed"}, 1.5},
		{"reviewer_run_cost_usd_total", labels{"project": "demo", "runner": "other", "status": "failed"}, 0},
	} {
		if got := counterValue(t, reg, c.metric, c.labels); got != c.want {
			t.Errorf("%s%v = %v, want %v", c.metric, c.labels, got, c.want)
		}
	}
	if n := seriesCount(t, reg, "reviewer_runs_total"); n != 2 {
		t.Errorf("reviewer_runs_total has %d series, want 2 (ok bundles and unknown projects not counted)", n)
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/debug/storage/", nil))
	page := rec.Body.String()
	for _, want := range []string{"demo", "billing", "$1.50", `class="st-failed"`} {
		if !strings.Contains(page, want) {
			t.Errorf("list page misses %q", want)
		}
	}
}

// counterValue returns the counter of metric with exactly these labels; 0 when absent.
func counterValue(t *testing.T, reg *prometheus.Registry, metric string, labels map[string]string) float64 {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != metric {
			continue
		}
		for _, m := range mf.GetMetric() {
			got := map[string]string{}
			for _, l := range m.GetLabel() {
				got[l.GetName()] = l.GetValue()
			}
			if maps.Equal(got, labels) {
				return m.GetCounter().GetValue()
			}
		}
	}
	return 0
}

// seriesCount returns how many label combinations metric has.
func seriesCount(t *testing.T, reg *prometheus.Registry, metric string) int {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() == metric {
			return len(mf.GetMetric())
		}
	}
	return 0
}

func TestHandler_UploadClipsMetadata(t *testing.T) {
	storage, e := newTestHandler(t)
	b := upload(t, storage, e, uuid.NewString(), map[string]string{
		FieldErrorMsg: strings.Repeat("e", maxErrorMsgBytes+100),
		FieldModel:    strings.Repeat("m", maxFieldBytes+100),
	})
	if len(b.ErrorMsg) != maxErrorMsgBytes || len(b.Model) != maxFieldBytes {
		t.Errorf("errorMsg %d bytes, model %d bytes: want %d and %d", len(b.ErrorMsg), len(b.Model), maxErrorMsgBytes, maxFieldBytes)
	}
}
