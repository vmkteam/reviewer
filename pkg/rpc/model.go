package rpc

import (
	"time"

	"reviewsrv/pkg/db"
	"reviewsrv/pkg/reviewer"
)

// IssueStats — статистика issues по severity.
type IssueStats struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Total    int `json:"total"`
}

// ModelInfo — информация о модели, выполнившей ревью.
type ModelInfo struct {
	Model        string  `json:"model"`
	InputTokens  int     `json:"inputTokens"`
	OutputTokens int     `json:"outputTokens"`
	CostUsd      float64 `json:"costUsd"`
}

func newIssueStats(in db.ReviewFileIssueStats) IssueStats {
	return IssueStats{
		Critical: in.Critical,
		High:     in.High,
		Medium:   in.Medium,
		Low:      in.Low,
		Total:    in.Total,
	}
}

func newModelInfo(in db.ReviewModelInfo) ModelInfo {
	return ModelInfo{
		Model:        in.Model,
		InputTokens:  in.InputTokens,
		OutputTokens: in.OutputTokens,
		CostUsd:      in.CostUsd,
	}
}

// Project — карточка проекта для /reviews/.
type Project struct {
	ID          int         `json:"projectId"`
	Title       string      `json:"title"`
	VcsURL      string      `json:"vcsURL"`
	Language    string      `json:"language"`
	CreatedAt   time.Time   `json:"createdAt"`
	ReviewCount int         `json:"reviewCount"`
	LastReview  *LastReview `json:"lastReview"`
}

// LastReview — краткая информация о последнем ревью проекта.
type LastReview struct {
	CreatedAt    time.Time `json:"createdAt"`
	Author       string    `json:"author"`
	TrafficLight string    `json:"trafficLight"`
}

func newProject(in *reviewer.Project) *Project {
	if in == nil {
		return nil
	}

	return &Project{
		ID:        in.ID,
		Title:     in.Title,
		VcsURL:    in.VcsURL,
		Language:  in.Language,
		CreatedAt: in.CreatedAt,
	}
}

// ReviewSummary — строка таблицы ревью для /reviews/project/<projectId>/.
type ReviewSummary struct {
	ID                  int                 `json:"reviewId"`
	Title               string              `json:"title"`
	ExternalID          string              `json:"externalId"`
	TrafficLight        string              `json:"trafficLight"`
	Author              string              `json:"author"`
	SourceBranch        string              `json:"sourceBranch"`
	TargetBranch        string              `json:"targetBranch"`
	CreatedAt           time.Time           `json:"createdAt"`
	ReviewFiles         []ReviewFileSummary `json:"reviewFiles"`
	LastVersionReviewID *int                `json:"lastVersionReviewId,omitempty"`
}

// ReviewFileSummary — мини-кружок A/C/S/T в таблице ревью.
type ReviewFileSummary struct {
	ReviewType   string     `json:"reviewType"`
	TrafficLight string     `json:"trafficLight"`
	IssueStats   IssueStats `json:"issueStats"`
}

func newReviewSummary(in *reviewer.Review) *ReviewSummary {
	if in == nil {
		return nil
	}

	rs := &ReviewSummary{
		ID:           in.ID,
		Title:        in.Title,
		ExternalID:   in.ExternalID,
		TrafficLight: in.TrafficLight,
		Author:       in.Author,
		SourceBranch: in.SourceBranch,
		TargetBranch: in.TargetBranch,
		CreatedAt:    in.CreatedAt,
		ReviewFiles:  make([]ReviewFileSummary, len(in.ReviewFiles)),
	}

	for i, rf := range in.ReviewFiles {
		rs.ReviewFiles[i] = ReviewFileSummary{
			ReviewType:   rf.ReviewType,
			TrafficLight: rf.TrafficLight,
			IssueStats:   newIssueStats(rf.IssueStats),
		}
	}

	rs.LastVersionReviewID = in.LastVersionReviewID

	return rs
}

// Review — полные данные ревью для /reviews/<reviewId>/.
type Review struct {
	ID                  int          `json:"reviewId"`
	ProjectID           int          `json:"projectId"`
	Title               string       `json:"title"`
	Description         string       `json:"description"`
	ExternalID          string       `json:"externalId"`
	TrafficLight        string       `json:"trafficLight"`
	CommitHash          string       `json:"commitHash"`
	SourceBranch        string       `json:"sourceBranch"`
	TargetBranch        string       `json:"targetBranch"`
	Author              string       `json:"author"`
	CreatedAt           time.Time    `json:"createdAt"`
	DurationMS          int          `json:"durationMs"`
	ModelInfo           ModelInfo    `json:"modelInfo"`
	ReviewFiles         []ReviewFile `json:"reviewFiles"`
	LastVersionReviewID *int         `json:"lastVersionReviewId,omitempty"`
}

func newReview(in *reviewer.Review) *Review {
	if in == nil {
		return nil
	}

	r := &Review{
		ID:                  in.ID,
		ProjectID:           in.ProjectID,
		Title:               in.Title,
		Description:         in.Description,
		ExternalID:          in.ExternalID,
		TrafficLight:        in.TrafficLight,
		CommitHash:          in.CommitHash,
		SourceBranch:        in.SourceBranch,
		TargetBranch:        in.TargetBranch,
		Author:              in.Author,
		CreatedAt:           in.CreatedAt,
		DurationMS:          in.DurationMS,
		ModelInfo:           newModelInfo(in.ModelInfo),
		ReviewFiles:         newReviewFiles(in.ReviewFiles),
		LastVersionReviewID: in.LastVersionReviewID,
	}

	return r
}

// ReviewFile — таб Architecture/Code/Security/Tests.
type ReviewFile struct {
	ID           int        `json:"reviewFileId"`
	ReviewType   string     `json:"reviewType"`
	TrafficLight string     `json:"trafficLight"`
	Summary      string     `json:"summary"`
	IssueStats   IssueStats `json:"issueStats"`
	Content      string     `json:"content"`
}

func newReviewFile(in *reviewer.ReviewFile) *ReviewFile {
	return &ReviewFile{
		ID:           in.ID,
		ReviewType:   in.ReviewType,
		TrafficLight: in.TrafficLight,
		Summary:      in.Summary,
		IssueStats:   newIssueStats(in.IssueStats),
		Content:      in.Content,
	}
}

// Issue — строка таблицы issues в табе Issues.
type Issue struct {
	ID              int     `json:"issueId"`
	ReviewID        int     `json:"reviewId"`
	Title           string  `json:"title"`
	Severity        string  `json:"severity"`
	Description     string  `json:"description"`
	Content         string  `json:"content"`
	File            string  `json:"file"`
	Lines           string  `json:"lines"`
	IssueType       string  `json:"issueType"`
	ReviewType      string  `json:"reviewType"`
	CommitHash      string  `json:"commitHash"`
	IsFalsePositive *bool   `json:"isFalsePositive"`
	Comment         *string `json:"comment"`
}

func newIssue(in *reviewer.Issue) *Issue {
	if in == nil {
		return nil
	}

	issue := &Issue{
		ID:              in.ID,
		ReviewID:        in.ReviewID,
		Title:           in.Title,
		Severity:        in.Severity,
		Description:     in.Description,
		Content:         in.Content,
		File:            in.File,
		Lines:           in.Lines,
		IssueType:       in.IssueType,
		ReviewType:      in.ReviewFile.ReviewType,
		IsFalsePositive: in.IsFalsePositive,
		Comment:         in.Comment,
	}

	if in.Review != nil {
		issue.CommitHash = in.Review.CommitHash
	}

	return issue
}

// ReviewFilters — фильтры для списка ревью.
type ReviewFilters struct {
	Author       *string `json:"author"`
	TrafficLight *string `json:"trafficLight"`
}

// ToDomain converts RPC filters to a domain ReviewSearch with pagination cursor.
func (f *ReviewFilters) ToDomain(projectID int, fromReviewID *int) *reviewer.ReviewSearch {
	s := &reviewer.ReviewSearch{
		ProjectID:    projectID,
		FromReviewID: fromReviewID,
	}
	if f != nil {
		s.Author = f.Author
		s.TrafficLight = f.TrafficLight
	}
	return s
}

// IssueFilters — фильтры для списка issues.
type IssueFilters struct {
	Severity        *string `json:"severity"`
	IssueType       *string `json:"issueType"`
	ReviewType      *string `json:"reviewType"`
	IsFalsePositive *bool   `json:"isFalsePositive"`
}

// ToDomain converts RPC filters to a domain IssueSearch scoped to a review.
func (f *IssueFilters) ToDomain(reviewID int) *reviewer.IssueSearch {
	s := &reviewer.IssueSearch{
		ReviewID: reviewID,
	}
	if f != nil {
		s.Severity = f.Severity
		s.IssueType = f.IssueType
		s.ReviewType = f.ReviewType
		s.IsFalsePositive = f.IsFalsePositive
	}
	return s
}

// ToDomainByProject converts RPC filters to a domain IssueSearch scoped to a project.
func (f *IssueFilters) ToDomainByProject(projectID int) *reviewer.IssueSearch {
	s := &reviewer.IssueSearch{
		ProjectID: &projectID,
	}
	if f != nil {
		s.Severity = f.Severity
		s.IssueType = f.IssueType
		s.ReviewType = f.ReviewType
		s.IsFalsePositive = f.IsFalsePositive
	}
	return s
}
