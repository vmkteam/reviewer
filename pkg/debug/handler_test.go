package debug

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func newTestHandler(t *testing.T) (*Storage, *echo.Echo) {
	t.Helper()
	storage := New(5)
	h := NewHandler(storage, slog.Default())

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
	if string(b.Files["review.json"]) != `{"files":[]}` {
		t.Errorf("review.json content mismatch: %q", b.Files["review.json"])
	}
	if string(b.Files["claude-output.json"]) != `{"type":"result"}` {
		t.Errorf("claude-output.json content mismatch: %q", b.Files["claude-output.json"])
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
		Files:        map[string][]byte{"review.json": []byte(`{"x":1}`)},
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
		Files: map[string][]byte{"review.json": []byte(`{"ok":true}`)},
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

func TestHandler_FileNotFound(t *testing.T) {
	_, e := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/debug/storage/missing/review.json", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
