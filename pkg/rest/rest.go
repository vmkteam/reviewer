package rest

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"text/template"

	"reviewsrv/pkg/db"
	"reviewsrv/pkg/reviewer"
	"reviewsrv/pkg/slack"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

//go:embed upload.js.tmpl
var uploadScriptTmpl string

var uploadScriptTemplate = template.Must(template.New("upload.js").Parse(uploadScriptTmpl))

type Handler struct {
	pm       *reviewer.ProjectManager
	rm       *reviewer.ReviewManager
	notifier *slack.Notifier
	baseURL  string
}

// NewHandler creates a REST handler with review management and Slack notifications.
func NewHandler(dbc db.DB, notifier *slack.Notifier, baseURL string) *Handler {
	return &Handler{
		pm:       reviewer.NewProjectManager(dbc),
		rm:       reviewer.NewReviewManager(dbc),
		notifier: notifier,
		baseURL:  baseURL,
	}
}

func (h *Handler) projectByKey(c echo.Context) (*reviewer.Project, error) {
	projectKey := c.Param("projectKey")
	if _, err := uuid.Parse(projectKey); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid project key")
	}

	project, err := h.pm.GetByKey(c.Request().Context(), projectKey)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if project == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "project key not found")
	}

	return project, nil
}

// CreateReview accepts a review draft via JSON, persists it, and sends a Slack notification.
func (h *Handler) CreateReview(c echo.Context) error {
	project, err := h.projectByKey(c)
	if err != nil {
		return err
	}

	var draft ReviewDraft
	if err = c.Bind(&draft); err != nil {
		return err
	}

	if err = draft.Validate(); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	model := draft.ToModel()
	rv, err := h.rm.CreateReview(c.Request().Context(), project, &model)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	h.notifySlack(project, rv)

	return c.String(http.StatusOK, strconv.Itoa(rv.ID))
}

func (h *Handler) notifySlack(project *reviewer.Project, rv *reviewer.Review) {
	if h.notifier == nil || !project.HasSlackWebhook() {
		return
	}

	var stats slack.IssueStats
	for _, rf := range rv.ReviewFiles {
		stats.Critical += rf.IssueStats.Critical
		stats.High += rf.IssueStats.High
		stats.Medium += rf.IssueStats.Medium
		stats.Low += rf.IssueStats.Low
	}

	notif := slack.ReviewNotification{
		WebhookURL:   project.SlackChannel.WebhookURL,
		ProjectTitle: project.Title,
		ReviewID:     rv.ID,
		Title:        rv.Title,
		Author:       rv.Author,
		SourceBranch: rv.SourceBranch,
		TargetBranch: rv.TargetBranch,
		TrafficLight: rv.TrafficLight,
		IssueStats:   stats,
		ReviewURL:    fmt.Sprintf("%s/reviews/%d/", h.baseURL, rv.ID),
	}

	go h.notifier.Send(context.Background(), notif)
}

// UploadReviewFile replaces the content of an existing review file from the request body.
func (h *Handler) UploadReviewFile(c echo.Context) error {
	project, err := h.projectByKey(c)
	if err != nil {
		return err
	}

	reviewID, err := strconv.Atoi(c.Param("reviewId"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid reviewId")
	}

	reviewType := c.Param("reviewType")
	rf, err := h.rm.ReviewFileByKey(c.Request().Context(), reviewID, reviewType, project.ID)
	if err != nil {
		if errors.Is(err, reviewer.ErrInvalidReviewType) {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if rf == nil {
		return echo.NewHTTPError(http.StatusNotFound, "review file not found")
	}

	content, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if _, err := h.rm.UpdateReviewFileContent(c.Request().Context(), rf, string(content)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusOK)
}

// GetUploadScript renders the JavaScript upload helper with the configured base URL.
func (h *Handler) GetUploadScript(c echo.Context) error {
	var buf bytes.Buffer
	if err := uploadScriptTemplate.Execute(&buf, map[string]string{"BaseURL": h.baseURL}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.Blob(http.StatusOK, "application/javascript", buf.Bytes())
}

// GetPrompt returns the assembled review prompt for the given project.
func (h *Handler) GetPrompt(c echo.Context) error {
	project, err := h.projectByKey(c)
	if err != nil {
		return err
	}

	prompt, err := h.pm.Prompt(c.Request().Context(), project.ProjectKey)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.String(http.StatusOK, prompt)
}
