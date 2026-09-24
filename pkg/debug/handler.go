package debug

import (
	"bytes"
	"compress/gzip"
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"math"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"reviewsrv/pkg/reviewer"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const (
	// maxFileBytes caps a single decompressed artifact (defends against gzip bombs).
	maxFileBytes = 32 * 1024 * 1024
	// maxErrorMsgBytes caps a bundle's error message; real ones are a few KB.
	maxErrorMsgBytes = 64 * 1024
	// maxFieldBytes caps the other metadata fields (ids, branches, model).
	maxFieldBytes = 1024
)

// StoragePathPrefix is the URL prefix under which bundle pages and files are served.
// Both clients (templates, reviewctl) and the upload handler reference it.
const StoragePathPrefix = "/v1/debug/storage/"

// Multipart form field names for the debug upload endpoint. Shared between
// reviewctl (writer) and the server handler (reader) to avoid drift.
const (
	FieldMRIid        = "mrIid"
	FieldExternalID   = "externalId"
	FieldRunner       = "runner"
	FieldModel        = "model"
	FieldErrorMsg     = "errorMsg"
	FieldSourceBranch = "sourceBranch"
	FieldTargetBranch = "targetBranch"
	FieldCommitHash   = "commitHash"
	FieldStatus       = "status"   // reviewer.RunStatus*
	FieldReason       = "reason"   // reviewer.RunReason*
	FieldCostUsd      = "costUsd"  // spent by the run, incl. a failed one
	FieldReviewID     = "reviewId" // set when the run's review was created before it failed
)

//go:embed templates/*.html
var templatesFS embed.FS

// Handler serves debug bundle endpoints over echo.
type Handler struct {
	storage      *Storage
	log          *slog.Logger
	tmpl         *template.Template
	projectTitle func(ctx context.Context, projectKey string) (string, error)
	metrics      *reviewer.RunMetrics
}

// NewHandler wires templates to the storage. Templates are embedded at compile
// time. projectTitle resolves an upload's project key to its title ("" for no
// project, which is refused) for the pages and the run metrics; metrics counts
// every uploaded run that did not complete (ok runs are counted on review
// upload). Either may be nil.
func NewHandler(storage *Storage, log *slog.Logger, projectTitle func(ctx context.Context, projectKey string) (string, error), metrics *reviewer.RunMetrics) *Handler {
	tmpl := template.Must(template.New("").Funcs(template.FuncMap{
		"shortKey":   shortKey,
		"preview":    preview,
		"storageURL": func() string { return StoragePathPrefix },
	}).ParseFS(templatesFS, "templates/*.html"))

	return &Handler{storage: storage, log: log, tmpl: tmpl, projectTitle: projectTitle, metrics: metrics}
}

// Upload accepts a multipart bundle. Each file part is gzip-compressed by the
// client and named "<original>.gz". Form fields carry run metadata.
func (h *Handler) Upload(c echo.Context) error {
	projectKey := c.Param("projectKey")
	if _, err := uuid.Parse(projectKey); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid project key")
	}
	ctx := c.Request().Context()
	var projectTitle string
	if h.projectTitle != nil {
		// Before the body is read: the route is unauthenticated, and a key that
		// matches no project could only fill the ring and the metrics with noise.
		title, err := h.projectTitle(ctx, projectKey)
		switch {
		case err != nil:
			h.log.ErrorContext(ctx, "debug upload: resolve project", "projectKey", shortKey(projectKey), "err", err)
			return echo.NewHTTPError(http.StatusServiceUnavailable, "resolve project")
		case title == "":
			return echo.NewHTTPError(http.StatusNotFound, "unknown project")
		}
		projectTitle = title
	}

	form, err := c.MultipartForm()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "parse multipart: "+err.Error())
	}

	// Metadata outlives the artifacts in the ring, so a client can't park
	// megabytes in it: fields are clipped.
	field := func(key string, n int) string {
		// Clone: a clipped substring would pin the whole posted value.
		return strings.Clone(reviewer.ClipUTF8(formValue(form.Value, key), n))
	}
	b := &Bundle{
		ProjectKey:   projectKey,
		ProjectTitle: projectTitle,
		MRIid:        field(FieldMRIid, maxFieldBytes),
		ExternalID:   field(FieldExternalID, maxFieldBytes),
		Runner:       field(FieldRunner, maxFieldBytes),
		Model:        field(FieldModel, maxFieldBytes),
		// The key authorizes project config (runner-profile and tracker
		// tokens): an upload URL quoted in an error must not reveal it.
		ErrorMsg:     strings.ReplaceAll(field(FieldErrorMsg, maxErrorMsgBytes), projectKey, shortKey(projectKey)+"…"),
		SourceBranch: field(FieldSourceBranch, maxFieldBytes),
		TargetBranch: field(FieldTargetBranch, maxFieldBytes),
		CommitHash:   field(FieldCommitHash, maxFieldBytes),
		CostUsd:      parseCost(formValue(form.Value, FieldCostUsd)),
		ReviewID:     reviewIDField(formValue(form.Value, FieldReviewID)),
		Files:        make(map[string]File, len(form.File)),
	}
	b.Status, b.Reason = reviewer.NormalizeRunOutcome(formValue(form.Value, FieldStatus), formValue(form.Value, FieldReason), b.ErrorMsg != "")

	for _, headers := range form.File {
		for _, fh := range headers {
			f, err := readArtifact(fh)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("read %s: %v", fh.Filename, err))
			}
			b.Files[strings.TrimSuffix(fh.Filename, ".gz")] = f
		}
	}

	h.storage.Add(b)
	// A failed or timed-out run is what an operator must notice; a cancelled
	// job or a --debug-upload of a good run is routine.
	lvl := slog.LevelInfo
	if b.Status == reviewer.RunStatusFailed || b.Status == reviewer.RunStatusTimeout {
		lvl = slog.LevelWarn
	}
	// A run whose review was created already counted as ok (with its cost)
	// on that upload: counting its later failure too would double both.
	counted := b.Status != reviewer.RunStatusOK && b.ReviewID == ""
	h.log.Log(ctx, lvl, "debug bundle stored",
		"id", b.ID, "projectKey", shortKey(projectKey), "project", b.ProjectTitle, "files", len(b.Files), "hasError", b.ErrorMsg != "",
		"status", b.Status, "reason", b.Reason, "runner", b.Runner, "model", b.Model, "costUsd", b.CostUsd,
		"reviewId", b.ReviewID, "countedInMetrics", counted, "errorMsg", preview(b.ErrorMsg, 300),
	)
	if counted {
		h.metrics.Observe(b.ProjectTitle, b.Runner, b.Status, b.Reason, b.CostUsd)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"id":  b.ID,
		"url": StoragePathPrefix + b.ID + "/",
	})
}

// List renders the index page.
func (h *Handler) List(c echo.Context) error {
	data := struct{ Bundles []*Bundle }{Bundles: h.storage.List()}
	return h.renderHTML(c, "list.html", data)
}

// Bundle renders the per-bundle page with metadata and a file table.
func (h *Handler) Bundle(c echo.Context) error {
	b := h.storage.Get(c.Param("id"))
	if b == nil {
		return echo.NewHTTPError(http.StatusNotFound, "bundle not found")
	}

	type fileEntry struct {
		Name string
		Size int
	}
	files := make([]fileEntry, 0, len(b.Files))
	for name, f := range b.Files {
		files = append(files, fileEntry{Name: name, Size: f.Size})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	data := struct {
		Bundle *Bundle
		Files  []fileEntry
	}{Bundle: b, Files: files}

	return h.renderHTML(c, "bundle.html", data)
}

// File serves a single artifact inline, as JSON or plain text in a sandbox
// (see contentTypeFor), decompressing it on the fly.
func (h *Handler) File(c echo.Context) error {
	f, ok := h.storage.GetFile(c.Param("id"), c.Param("filename"))
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "file not found")
	}
	gr, err := gzip.NewReader(bytes.NewReader(f.Gzip))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "gzip: "+err.Error())
	}
	defer gr.Close()
	// Belt and braces for uploaded content: no sniffing into a renderable type,
	// and no scripts even if a browser renders it anyway.
	hdr := c.Response().Header()
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Content-Security-Policy", "sandbox")
	return c.Stream(http.StatusOK, contentTypeFor(c.Param("filename")), gr)
}

func (h *Handler) renderHTML(c echo.Context, name string, data any) error {
	var sb strings.Builder
	if err := h.tmpl.ExecuteTemplate(&sb, name, data); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "render "+name+": "+err.Error())
	}
	return c.HTML(http.StatusOK, sb.String())
}

// reviewIDField keeps a client-reported review id only when it is one, so a
// garbage value can't switch off the run metrics.
func reviewIDField(s string) string {
	id, err := strconv.Atoi(s)
	if err != nil || id <= 0 {
		return ""
	}
	return strconv.Itoa(id)
}

// parseCost reads a client-reported dollar cost; anything unusable is 0.
func parseCost(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

func formValue(m map[string][]string, key string) string {
	if v, ok := m[key]; ok && len(v) > 0 {
		return v[0]
	}
	return ""
}

// readArtifact reads a gzip-compressed multipart file and checks that it
// decompresses within maxFileBytes, so a gzip bomb is rejected at upload. The
// compressed bytes are kept as they are.
func readArtifact(fh *multipart.FileHeader) (File, error) {
	f, err := fh.Open()
	if err != nil {
		return File{}, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	raw, err := io.ReadAll(f)
	if err != nil {
		return File{}, fmt.Errorf("read: %w", err)
	}
	gr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return File{}, fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()
	n, err := io.Copy(io.Discard, io.LimitReader(gr, maxFileBytes+1))
	if err != nil {
		return File{}, fmt.Errorf("gunzip: %w", err)
	}
	if n > maxFileBytes {
		return File{}, errors.New("artifact exceeds size limit")
	}
	return File{Gzip: raw, Size: int(n)}, nil
}

// contentTypeFor picks the Content-Type an artifact is served with: JSON as
// such, anything else as plain text. The name comes from the uploader, and the
// debug pages share an origin with the admin panel, so no type that a browser
// would render as a document (html, svg) is ever derived from it.
func contentTypeFor(name string) string {
	if strings.EqualFold(filepath.Ext(name), ".json") {
		return "application/json"
	}
	return "text/plain; charset=utf-8"
}

// shortKey returns the first 8 chars of a UUID for compact display.
func shortKey(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}

// preview truncates a string to n bytes, on a rune boundary, for the index
// error column and the log.
func preview(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return reviewer.ClipUTF8(s, n) + "…"
}
