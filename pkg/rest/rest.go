package rest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"reviewsrv/pkg/db"
	"reviewsrv/pkg/reviewer"
	"reviewsrv/pkg/slack"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

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

// ReviewFixMarkdown returns a markdown document listing valid issues of a review,
// intended to be consumed by Claude Code as a fix task prompt.
// URL contract: /v1/rpc/review-fix-<id>.md — .md suffix is required.
func (h *Handler) ReviewFixMarkdown(c echo.Context) error {
	param := c.Param("id")
	if !strings.HasSuffix(param, ".md") {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid review id")
	}
	reviewID, err := strconv.Atoi(strings.TrimSuffix(param, ".md"))
	if err != nil || reviewID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid review id")
	}

	md, err := h.rm.RenderFixMarkdown(c.Request().Context(), h.pm, reviewID)
	if err != nil {
		if errors.Is(err, reviewer.ErrReviewNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "review not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.Blob(http.StatusOK, "text/markdown; charset=utf-8", []byte(md))
}

// ProjectInstructionsMarkdown returns a markdown document listing non-archived
// ignored issues of a project, intended to be consumed by an LLM that synthesizes
// project-specific review rules.
// URL contract: /v1/rpc/project-instructions-<id>.md — .md suffix is required.
func (h *Handler) ProjectInstructionsMarkdown(c echo.Context) error {
	param := c.Param("id")
	if !strings.HasSuffix(param, ".md") {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid project id")
	}
	projectID, err := strconv.Atoi(strings.TrimSuffix(param, ".md"))
	if err != nil || projectID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid project id")
	}

	md, err := h.rm.RenderProjectInstructionsMarkdown(c.Request().Context(), h.pm, projectID)
	if err != nil {
		if errors.Is(err, reviewer.ErrProjectNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "project not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.Blob(http.StatusOK, "text/markdown; charset=utf-8", []byte(md))
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
