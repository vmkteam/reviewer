package debug

import (
	"compress/gzip"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// maxFileBytes caps a single decompressed artifact (defends against gzip bombs).
const maxFileBytes = 32 * 1024 * 1024

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
)

//go:embed templates/*.html
var templatesFS embed.FS

// Handler serves debug bundle endpoints over echo.
type Handler struct {
	storage *Storage
	log     *slog.Logger
	tmpl    *template.Template
}

// NewHandler wires templates to the storage. Templates are embedded at compile time.
func NewHandler(storage *Storage, log *slog.Logger) *Handler {
	tmpl := template.Must(template.New("").Funcs(template.FuncMap{
		"shortKey":   shortKey,
		"preview":    preview,
		"storageURL": func() string { return StoragePathPrefix },
	}).ParseFS(templatesFS, "templates/*.html"))

	return &Handler{storage: storage, log: log, tmpl: tmpl}
}

// Upload accepts a multipart bundle. Each file part is gzip-compressed by the
// client and named "<original>.gz". Form fields carry run metadata.
func (h *Handler) Upload(c echo.Context) error {
	projectKey := c.Param("projectKey")
	if _, err := uuid.Parse(projectKey); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid project key")
	}

	form, err := c.MultipartForm()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "parse multipart: "+err.Error())
	}

	b := &Bundle{
		ProjectKey:   projectKey,
		MRIid:        formValue(form.Value, FieldMRIid),
		ExternalID:   formValue(form.Value, FieldExternalID),
		Runner:       formValue(form.Value, FieldRunner),
		Model:        formValue(form.Value, FieldModel),
		ErrorMsg:     formValue(form.Value, FieldErrorMsg),
		SourceBranch: formValue(form.Value, FieldSourceBranch),
		TargetBranch: formValue(form.Value, FieldTargetBranch),
		CommitHash:   formValue(form.Value, FieldCommitHash),
		Files:        make(map[string][]byte, len(form.File)),
	}

	for _, headers := range form.File {
		for _, fh := range headers {
			data, err := readGzipped(fh)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("read %s: %v", fh.Filename, err))
			}
			name := strings.TrimSuffix(fh.Filename, ".gz")
			b.Files[name] = data
		}
	}

	h.storage.Add(b)
	h.log.InfoContext(c.Request().Context(), "debug bundle stored",
		"id", b.ID, "projectKey", projectKey, "files", len(b.Files), "hasError", b.ErrorMsg != "",
	)

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
	for name, data := range b.Files {
		files = append(files, fileEntry{Name: name, Size: len(data)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	data := struct {
		Bundle *Bundle
		Files  []fileEntry
	}{Bundle: b, Files: files}

	return h.renderHTML(c, "bundle.html", data)
}

// File serves a single artifact inline so the browser can render it.
func (h *Handler) File(c echo.Context) error {
	data, ok := h.storage.GetFile(c.Param("id"), c.Param("filename"))
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "file not found")
	}
	return c.Blob(http.StatusOK, contentTypeFor(c.Param("filename")), data)
}

func (h *Handler) renderHTML(c echo.Context, name string, data any) error {
	var sb strings.Builder
	if err := h.tmpl.ExecuteTemplate(&sb, name, data); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "render "+name+": "+err.Error())
	}
	return c.HTML(http.StatusOK, sb.String())
}

func formValue(m map[string][]string, key string) string {
	if v, ok := m[key]; ok && len(v) > 0 {
		return v[0]
	}
	return ""
}

// readGzipped opens the multipart file, gunzips it, and returns the raw bytes.
// Caps at maxFileBytes to prevent a malicious gzip from exhausting memory.
func readGzipped(fh *multipart.FileHeader) ([]byte, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()

	data, err := io.ReadAll(io.LimitReader(gr, maxFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	if len(data) > maxFileBytes {
		return nil, errors.New("artifact exceeds size limit")
	}
	return data, nil
}

// contentTypeFor picks a browser-friendly Content-Type by extension.
// Markdown and JSONL render best as text/plain so the browser shows them inline
// rather than offering a download or rendering as raw markdown.
func contentTypeFor(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".jsonl", ".log":
		return "text/plain; charset=utf-8"
	}
	if ct := mime.TypeByExtension(filepath.Ext(name)); ct != "" {
		return ct
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

// preview truncates a string to n runes for the index error column.
func preview(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
