package ctl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
		respBody, _ := io.ReadAll(resp.Body)
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
func ReadReviewJSON(dir string) (*rest.ReviewDraft, error) {
	path := filepath.Join(dir, "review.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read review.json: %w", err)
	}

	var draft rest.ReviewDraft
	if err := json.Unmarshal(data, &draft); err != nil {
		return nil, fmt.Errorf("parse review.json: %w", err)
	}

	if err := draft.Validate(); err != nil {
		return nil, fmt.Errorf("validate review.json: %w", err)
	}

	return &draft, nil
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
