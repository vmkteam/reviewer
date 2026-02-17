package reviewer

import (
	"errors"
	"slices"
	"time"

	"reviewsrv/pkg/db"

	"github.com/go-pg/pg/v10"
)

const (
	ReviewTypeArchitecture = "architecture"
	ReviewTypeCode         = "code"
	ReviewTypeSecurity     = "security"
	ReviewTypeTests        = "tests"

	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
)

var (
	ReviewTypes            = []string{ReviewTypeArchitecture, ReviewTypeCode, ReviewTypeSecurity, ReviewTypeTests}
	Severities             = []string{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow}
	ErrInvalidReviewType   = errors.New("invalid review type")
	ErrDuplicateReviewType = errors.New("duplicate review type")
)

// IsValidReviewType checks if the given review type is supported.
func IsValidReviewType(rt string) bool {
	return slices.Contains(ReviewTypes, rt)
}

// IsValidSeverity checks if the given severity is supported.
func IsValidSeverity(s string) bool {
	return slices.Contains(Severities, s)
}

type Review struct {
	db.Review
	ReviewFiles         ReviewFiles
	LastVersionReviewID *int
}

// NewReview converts a db.Review to the domain model, returning nil for nil input.
func NewReview(in *db.Review) *Review {
	if in == nil {
		return nil
	}

	return &Review{
		Review: *in,
	}
}

type ReviewFile struct {
	db.ReviewFile
	Issues Issues
}

// NewReviewFile converts a db.ReviewFile to the domain model, returning nil for nil input.
func NewReviewFile(in *db.ReviewFile) *ReviewFile {
	if in == nil {
		return nil
	}

	return &ReviewFile{
		ReviewFile: *in,
	}
}

type Issue struct {
	db.Issue
}

// NewIssue converts a db.Issue to the domain model, returning nil for nil input.
func NewIssue(in *db.Issue) *Issue {
	if in == nil {
		return nil
	}

	return &Issue{
		Issue: *in,
	}
}

type Project struct {
	db.Project
}

// NewProject converts a db.Project to the domain model, returning nil for nil input.
func NewProject(in *db.Project) *Project {
	if in == nil {
		return nil
	}

	return &Project{
		Project: *in,
	}
}

// HasSlackWebhook returns true if the project has a configured Slack webhook.
func (p *Project) HasSlackWebhook() bool {
	return p.SlackChannel != nil && p.SlackChannel.WebhookURL != ""
}

type TaskTracker struct {
	db.TaskTracker
}

// NewTaskTracker converts a db.TaskTracker to the domain model, returning nil for nil input.
func NewTaskTracker(in *db.TaskTracker) *TaskTracker {
	if in == nil {
		return nil
	}

	return &TaskTracker{
		TaskTracker: *in,
	}
}

type Prompt struct {
	db.Prompt
}

// NewPrompt converts a db.Prompt to the domain model, returning nil for nil input.
func NewPrompt(in *db.Prompt) *Prompt {
	if in == nil {
		return nil
	}

	return &Prompt{
		Prompt: *in,
	}
}

type IssueStats db.ReviewFileIssueStats

// Add accumulates severity counters from another IssueStats.
func (s *IssueStats) Add(other IssueStats) {
	s.Critical += other.Critical
	s.High += other.High
	s.Medium += other.Medium
	s.Low += other.Low
	s.Total += other.Total
}

func calcIssueStats(issues Issues) IssueStats {
	var s IssueStats
	for _, iss := range issues {
		switch iss.Severity {
		case "critical":
			s.Critical++
		case "high":
			s.High++
		case "medium":
			s.Medium++
		case "low":
			s.Low++
		}
	}
	s.Total = s.Critical + s.High + s.Medium + s.Low
	return s
}

func calcTrafficLight(s IssueStats) string {
	switch {
	case s.Critical >= 1 || s.High >= 2:
		return "red"
	case s.High >= 1 || s.Medium >= 3:
		return "yellow"
	default:
		return "green"
	}
}

// ProjectStats contains aggregated review stats per project.
type ProjectStats struct {
	ProjectID    int       `pg:"projectId"`
	ReviewCount  int       `pg:"reviewCount"`
	CreatedAt    time.Time `pg:"createdAt"`
	Author       string    `pg:"author"`
	TrafficLight string    `pg:"trafficLight"`
}

// ReviewSearch contains search params for listing reviews.
type ReviewSearch struct {
	ProjectID    int
	Author       *string
	TrafficLight *string
	FromReviewID *int
}

// ToDB converts domain search params to the database layer representation.
func (s *ReviewSearch) ToDB() *db.ReviewSearch {
	if s == nil {
		return nil
	}

	search := &db.ReviewSearch{
		ProjectID:    &s.ProjectID,
		Author:       s.Author,
		TrafficLight: s.TrafficLight,
		IDLt:         s.FromReviewID,
	}
	return search
}

// IssueSearch contains search params for listing issues.
type IssueSearch struct {
	ReviewID        int
	ProjectID       *int
	IsFalsePositive *bool
	FromIssueID     *int
	Severity        *string
	IssueType       *string
	ReviewType      *string
}

// ToDB converts domain search params to the database layer representation.
func (s *IssueSearch) ToDB() *db.IssueSearch {
	if s == nil {
		return nil
	}

	search := &db.IssueSearch{
		Severity:             s.Severity,
		IssueType:            s.IssueType,
		ReviewFileReviewType: s.ReviewType,
		IsFalsePositive:      s.IsFalsePositive,
		ReviewProjectID:      s.ProjectID,
	}
	if s.ReviewID != 0 {
		search.ReviewID = &s.ReviewID
	}
	if s.FromIssueID != nil {
		search.With("?.? > ?", pg.Ident("t"), pg.Ident(db.Columns.Issue.ID), *s.FromIssueID)
	}
	return search
}

type SlackChannel struct {
	db.SlackChannel
}

// NewSlackChannel converts a db.SlackChannel to the domain model, returning nil for nil input.
func NewSlackChannel(in *db.SlackChannel) *SlackChannel {
	if in == nil {
		return nil
	}

	return &SlackChannel{
		SlackChannel: *in,
	}
}
