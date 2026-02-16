package reviewer

import (
	"testing"

	"reviewsrv/pkg/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsValidReviewType(t *testing.T) {
	tests := []struct {
		name string
		rt   string
		want bool
	}{
		{"architecture", "architecture", true},
		{"code", "code", true},
		{"security", "security", true},
		{"tests", "tests", true},
		{"empty", "", false},
		{"unknown", "unknown", false},
		{"case sensitive", "Architecture", false},
		{"uppercase", "CODE", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsValidReviewType(tt.rt))
		})
	}
}

func TestIsValidSeverity(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"critical", "critical", true},
		{"high", "high", true},
		{"medium", "medium", true},
		{"low", "low", true},
		{"empty", "", false},
		{"info", "info", false},
		{"case sensitive", "Critical", false},
		{"uppercase", "HIGH", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsValidSeverity(tt.s))
		})
	}
}

func TestCalcIssueStats(t *testing.T) {
	tests := []struct {
		name string
		in   Issues
		want IssueStats
	}{
		{
			name: "empty",
			in:   nil,
			want: IssueStats{},
		},
		{
			name: "mixed severities",
			in: Issues{
				{db.Issue{Severity: "critical"}},
				{db.Issue{Severity: "high"}},
				{db.Issue{Severity: "high"}},
				{db.Issue{Severity: "medium"}},
				{db.Issue{Severity: "low"}},
				{db.Issue{Severity: "low"}},
				{db.Issue{Severity: "low"}},
			},
			want: IssueStats{Critical: 1, High: 2, Medium: 1, Low: 3, Total: 7},
		},
		{
			name: "unknown severity ignored",
			in: Issues{
				{db.Issue{Severity: "critical"}},
				{db.Issue{Severity: "unknown"}},
				{db.Issue{Severity: "info"}},
			},
			want: IssueStats{Critical: 1, Total: 1},
		},
		{
			name: "all critical",
			in: Issues{
				{db.Issue{Severity: "critical"}},
				{db.Issue{Severity: "critical"}},
			},
			want: IssueStats{Critical: 2, Total: 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, calcIssueStats(tt.in))
		})
	}
}

func TestCalcTrafficLight(t *testing.T) {
	tests := []struct {
		name string
		in   IssueStats
		want string
	}{
		{"zero stats", IssueStats{}, "green"},
		{"1 critical = red", IssueStats{Critical: 1, Total: 1}, "red"},
		{"2 high = red", IssueStats{High: 2, Total: 2}, "red"},
		{"1 high = yellow", IssueStats{High: 1, Total: 1}, "yellow"},
		{"3 medium = yellow", IssueStats{Medium: 3, Total: 3}, "yellow"},
		{"2 medium = green", IssueStats{Medium: 2, Total: 2}, "green"},
		{"1 low = green", IssueStats{Low: 1, Total: 1}, "green"},
		{"1 critical + 2 high = red", IssueStats{Critical: 1, High: 2, Total: 3}, "red"},
		{"1 high + 3 medium = yellow (high wins)", IssueStats{High: 1, Medium: 3, Total: 4}, "yellow"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, calcTrafficLight(tt.in))
		})
	}
}

func TestIssueStats_Add(t *testing.T) {
	tests := []struct {
		name  string
		base  IssueStats
		other IssueStats
		want  IssueStats
	}{
		{
			name:  "zero + zero",
			base:  IssueStats{},
			other: IssueStats{},
			want:  IssueStats{},
		},
		{
			name:  "accumulate",
			base:  IssueStats{Critical: 1, High: 2, Medium: 3, Low: 4, Total: 10},
			other: IssueStats{Critical: 1, High: 1, Medium: 1, Low: 1, Total: 4},
			want:  IssueStats{Critical: 2, High: 3, Medium: 4, Low: 5, Total: 14},
		},
		{
			name:  "add to zero",
			base:  IssueStats{},
			other: IssueStats{Critical: 5, Total: 5},
			want:  IssueStats{Critical: 5, Total: 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.base
			s.Add(tt.other)
			assert.Equal(t, tt.want, s)
		})
	}
}

func TestPrepareReview(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		pr := &Project{db.Project{ID: 10, PromptID: 20, ProjectKey: "test-key"}}
		rv := &Review{
			ReviewFiles: ReviewFiles{
				{
					ReviewFile: db.ReviewFile{ReviewType: "code"},
					Issues: Issues{
						{db.Issue{Severity: "high"}},
						{db.Issue{Severity: "low"}},
					},
				},
				{
					ReviewFile: db.ReviewFile{ReviewType: "security"},
					Issues: Issues{
						{db.Issue{Severity: "critical"}},
					},
				},
			},
		}

		err := prepareReview(pr, rv)
		require.NoError(t, err)

		assert.Equal(t, 10, rv.ProjectID)
		assert.Equal(t, 20, rv.PromptID)
		assert.Equal(t, db.StatusEnabled, rv.StatusID)

		// code file: 1 high + 1 low
		assert.Equal(t, db.ReviewFileIssueStats{High: 1, Low: 1, Total: 2}, rv.ReviewFiles[0].IssueStats)
		assert.Equal(t, "yellow", rv.ReviewFiles[0].TrafficLight)
		assert.Equal(t, db.StatusEnabled, rv.ReviewFiles[0].StatusID)

		// security file: 1 critical
		assert.Equal(t, db.ReviewFileIssueStats{Critical: 1, Total: 1}, rv.ReviewFiles[1].IssueStats)
		assert.Equal(t, "red", rv.ReviewFiles[1].TrafficLight)
		assert.Equal(t, db.StatusEnabled, rv.ReviewFiles[1].StatusID)

		// total: 1 critical + 1 high + 1 low = red
		assert.Equal(t, "red", rv.TrafficLight)

		// issues get statusID
		for _, rf := range rv.ReviewFiles {
			for _, iss := range rf.Issues {
				assert.Equal(t, db.StatusEnabled, iss.StatusID)
			}
		}
	})

	t.Run("duplicate reviewType", func(t *testing.T) {
		pr := &Project{db.Project{ID: 1, PromptID: 1}}
		rv := &Review{
			ReviewFiles: ReviewFiles{
				{ReviewFile: db.ReviewFile{ReviewType: "code"}},
				{ReviewFile: db.ReviewFile{ReviewType: "code"}},
			},
		}

		err := prepareReview(pr, rv)
		assert.ErrorIs(t, err, ErrDuplicateReviewType)
	})

	t.Run("empty review files", func(t *testing.T) {
		pr := &Project{db.Project{ID: 1, PromptID: 1}}
		rv := &Review{}

		err := prepareReview(pr, rv)
		require.NoError(t, err)
		assert.Equal(t, "green", rv.TrafficLight)
	})
}

func TestHasSlackWebhook(t *testing.T) {
	tests := []struct {
		name string
		p    *Project
		want bool
	}{
		{
			name: "with webhook",
			p:    &Project{db.Project{SlackChannel: &db.SlackChannel{WebhookURL: "https://hooks.slack.com/xxx"}}},
			want: true,
		},
		{
			name: "empty webhook",
			p:    &Project{db.Project{SlackChannel: &db.SlackChannel{WebhookURL: ""}}},
			want: false,
		},
		{
			name: "nil slack channel",
			p:    &Project{db.Project{}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.p.HasSlackWebhook())
		})
	}
}

func TestReviewSearch_ToDB(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var s *ReviewSearch
		assert.Nil(t, s.ToDB())
	})

	t.Run("full mapping", func(t *testing.T) {
		author := "john"
		tl := "green"
		fromID := 42
		s := &ReviewSearch{
			ProjectID:    10,
			Author:       &author,
			TrafficLight: &tl,
			FromReviewID: &fromID,
		}

		got := s.ToDB()
		assert.NotNil(t, got)
		assert.Equal(t, 10, *got.ProjectID)
		assert.Equal(t, "john", *got.Author)
		assert.Equal(t, "green", *got.TrafficLight)
		assert.Equal(t, 42, *got.IDLt)
	})

	t.Run("minimal", func(t *testing.T) {
		s := &ReviewSearch{ProjectID: 5}

		got := s.ToDB()
		assert.Equal(t, 5, *got.ProjectID)
		assert.Nil(t, got.Author)
		assert.Nil(t, got.TrafficLight)
		assert.Nil(t, got.IDLt)
	})
}

func TestIssueSearch_ToDB(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var s *IssueSearch
		assert.Nil(t, s.ToDB())
	})

	t.Run("full mapping", func(t *testing.T) {
		severity := "high"
		issueType := "error-handling"
		reviewType := "code"
		isFP := true
		projectID := 5
		fromID := 100
		s := &IssueSearch{
			ReviewID:        10,
			ProjectID:       &projectID,
			IsFalsePositive: &isFP,
			FromIssueID:     &fromID,
			Severity:        &severity,
			IssueType:       &issueType,
			ReviewType:      &reviewType,
		}

		got := s.ToDB()
		assert.NotNil(t, got)
		assert.Equal(t, 10, *got.ReviewID)
		assert.Equal(t, "high", *got.Severity)
		assert.Equal(t, "error-handling", *got.IssueType)
		assert.Equal(t, "code", *got.ReviewFileReviewType)
		assert.True(t, *got.IsFalsePositive)
		assert.Equal(t, 5, *got.ReviewProjectID)
	})

	t.Run("zero reviewID not set", func(t *testing.T) {
		s := &IssueSearch{ReviewID: 0}

		got := s.ToDB()
		assert.Nil(t, got.ReviewID)
	})

	t.Run("non-zero reviewID set", func(t *testing.T) {
		s := &IssueSearch{ReviewID: 7}

		got := s.ToDB()
		assert.Equal(t, 7, *got.ReviewID)
	})
}
