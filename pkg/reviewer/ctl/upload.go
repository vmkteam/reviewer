package ctl

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"reviewsrv/pkg/debug"
	"reviewsrv/pkg/rest"
)

// reviewTypeByPrefix maps R*.md file prefixes to review types.
var reviewTypeByPrefix = map[string]string{
	"R1": "architecture",
	"R2": "code",
	"R3": "security",
	"R4": "tests",
	"R5": "operability",
}

// UploadClient uploads review data to the reviewsrv server.
type UploadClient struct {
	httpClient *http.Client
	log        *slog.Logger
}

// NewUploadClient creates a new UploadClient.
func NewUploadClient(log *slog.Logger) *UploadClient {
	return &UploadClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		log:        log,
	}
}

// UploadReview uploads review.json and returns the reviewId.
func (c *UploadClient) UploadReview(ctx context.Context, serverURL, projectKey string, draft *rest.ReviewDraft) (int, error) {
	url := fmt.Sprintf("%s/v1/upload/%s/", strings.TrimRight(serverURL, "/"), projectKey)

	body, err := json.Marshal(draft)
	if err != nil {
		return 0, fmt.Errorf("marshal review draft: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("upload review: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read upload response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("upload review: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	reviewID, err := strconv.Atoi(strings.TrimSpace(string(respBody)))
	if err != nil {
		return 0, fmt.Errorf("parse reviewId: %w", err)
	}

	c.log.InfoContext(ctx, "uploaded review", "reviewId", reviewID)

	return reviewID, nil
}

// UploadFile uploads a single review file (markdown content).
func (c *UploadClient) UploadFile(ctx context.Context, serverURL, projectKey string, reviewID int, reviewType string, content []byte) error {
	url := fmt.Sprintf("%s/v1/upload/%s/%d/%s/", strings.TrimRight(serverURL, "/"), projectKey, reviewID, reviewType)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("create file upload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload file %s: %w", reviewType, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("upload file %s: HTTP %d: %s", reviewType, resp.StatusCode, string(respBody))
	}

	c.log.InfoContext(ctx, "uploaded file", "reviewType", reviewType)

	return nil
}

// UploadAll uploads review.json and all R*.md files.
func (c *UploadClient) UploadAll(ctx context.Context, serverURL, projectKey string, draft *rest.ReviewDraft, mdFiles map[string]string) (int, error) {
	reviewID, err := c.UploadReview(ctx, serverURL, projectKey, draft)
	if err != nil {
		return 0, err
	}

	for reviewType, filePath := range mdFiles {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return reviewID, fmt.Errorf("read %s: %w", filePath, err)
		}
		if err := c.UploadFile(ctx, serverURL, projectKey, reviewID, reviewType, content); err != nil {
			return reviewID, err
		}
	}

	return reviewID, nil
}

// ReadReviewJSON reads and validates review.json from the given directory.
// On validation failure, also returns the parsed draft so the caller can
// surface diagnostic detail without re-reading the file.
func ReadReviewJSON(dir string) (*rest.ReviewDraft, error) {
	path := filepath.Join(dir, "review.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read review.json: %w", err)
	}

	var draft rest.ReviewDraft
	if err := json.Unmarshal(data, &draft); err != nil {
		return nil, fmt.Errorf("parse review.json (size=%d): %w", len(data), err)
	}

	if err := draft.Validate(); err != nil {
		return &draft, fmt.Errorf("validate review.json: %w", err)
	}

	return &draft, nil
}

// DebugMeta carries reviewctl run metadata uploaded alongside artifacts.
type DebugMeta struct {
	MRIid        string
	ExternalID   string
	Runner       string
	Model        string
	ErrorMsg     string
	SourceBranch string
	TargetBranch string
	CommitHash   string
}

// UploadDebugBundle posts artifacts as a multipart form with each file
// gzip-compressed in its own part. Returns the bundle URL reported by the server.
func (c *UploadClient) UploadDebugBundle(ctx context.Context, serverURL, projectKey string, meta DebugMeta, files map[string][]byte) (string, error) {
	if len(files) == 0 && meta.ErrorMsg == "" {
		return "", nil
	}

	body, contentType, err := buildDebugMultipart(meta, files)
	if err != nil {
		return "", fmt.Errorf("build multipart: %w", err)
	}

	url := fmt.Sprintf("%s/v1/upload/debug/%s/", strings.TrimRight(serverURL, "/"), projectKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return "", fmt.Errorf("create debug upload request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("post debug bundle: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("debug upload: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var out struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("parse debug response: %w", err)
	}
	return out.URL, nil
}

func buildDebugMultipart(meta DebugMeta, files map[string][]byte) (io.Reader, string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// Sorted iteration keeps the wire body stable, which matters for
	// reproducible test fixtures and easier diffing of captured requests.
	fields := []struct {
		name, value string
	}{
		{debug.FieldMRIid, meta.MRIid},
		{debug.FieldExternalID, meta.ExternalID},
		{debug.FieldRunner, meta.Runner},
		{debug.FieldModel, meta.Model},
		{debug.FieldErrorMsg, meta.ErrorMsg},
		{debug.FieldSourceBranch, meta.SourceBranch},
		{debug.FieldTargetBranch, meta.TargetBranch},
		{debug.FieldCommitHash, meta.CommitHash},
	}
	for _, f := range fields {
		if f.value == "" {
			continue
		}
		if err := mw.WriteField(f.name, f.value); err != nil {
			return nil, "", fmt.Errorf("write field %s: %w", f.name, err)
		}
	}

	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		part, err := mw.CreateFormFile("file", name+".gz")
		if err != nil {
			return nil, "", fmt.Errorf("create part %s: %w", name, err)
		}
		gw := gzip.NewWriter(part)
		if _, err := gw.Write(files[name]); err != nil {
			return nil, "", fmt.Errorf("gzip %s: %w", name, err)
		}
		if err := gw.Close(); err != nil {
			return nil, "", fmt.Errorf("close gzip %s: %w", name, err)
		}
	}

	if err := mw.Close(); err != nil {
		return nil, "", fmt.Errorf("close multipart: %w", err)
	}
	return &buf, mw.FormDataContentType(), nil
}

// CollectDebugArtifacts reads the artifacts that reviewctl writes during a run.
// Missing files are silently skipped — the caller wants whatever is on disk.
func CollectDebugArtifacts(dir string) map[string][]byte {
	candidates := []string{"claude-output.json", "opencode-output.jsonl", "review.json"}
	out := make(map[string][]byte, len(candidates)+len(reviewTypeByPrefix))

	for _, name := range candidates {
		if data, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			out[name] = data
		}
	}

	mdFiles, err := FindMDFiles(dir)
	if err != nil {
		return out
	}
	for _, path := range mdFiles {
		if data, err := os.ReadFile(path); err == nil {
			out[filepath.Base(path)] = data
		}
	}

	return out
}

// FindMDFiles scans the directory for R*.md files and returns a map of reviewType → filepath.
func FindMDFiles(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	result := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		for prefix, reviewType := range reviewTypeByPrefix {
			if strings.HasPrefix(name, prefix+".") {
				result[reviewType] = filepath.Join(dir, name)
				break
			}
		}
	}

	return result, nil
}
