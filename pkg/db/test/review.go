//nolint:dupl,funlen
package test

import (
	"context"
	"testing"
	"time"

	"reviewsrv/pkg/db"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/go-pg/pg/v10/orm"
)

type IssueOpFunc func(t *testing.T, dbo orm.DB, in *db.Issue) Cleaner

func Issue(t *testing.T, dbo orm.DB, in *db.Issue, ops ...IssueOpFunc) (*db.Issue, Cleaner) {
	repo := db.NewReviewRepo(dbo)
	var cleaners []Cleaner

	// Fill the incoming entity
	if in == nil {
		in = &db.Issue{}
	}

	// Check if PKs are provided
	if in.ID != 0 {
		// Fetch the entity by PK
		issue, err := repo.IssueByID(t.Context(), in.ID, repo.FullIssue())
		if err != nil {
			t.Fatal(err)
		}

		// We must find the entity by PK
		if issue == nil {
			t.Fatalf("the entity Issue is not found by provided PKs ID=%v", in.ID)
		}

		// Return if found without real cleanup
		return issue, emptyClean
	}

	for _, op := range ops {
		if cl := op(t, dbo, in); cl != nil {
			cleaners = append(cleaners, cl)
		}
	}

	// Create the main entity
	issue, err := repo.AddIssue(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}

	return issue, func() {
		if _, err := dbo.ModelContext(context.Background(), &db.Issue{ID: issue.ID}).WherePK().Delete(); err != nil {
			t.Fatal(err)
		}
		// Clean up related entities from the last to the first
		for i := len(cleaners) - 1; i >= 0; i-- {
			cleaners[i]()
		}
	}
}

func WithIssueRelations(t *testing.T, dbo orm.DB, in *db.Issue) Cleaner {
	var cleaners []Cleaner

	// Prepare main relations
	if in.ReviewFile == nil {
		in.ReviewFile = &db.ReviewFile{}
	}

	// Prepare nested relations which have the same relations
	if in.ReviewFile.Review == nil {
		in.ReviewFile.Review = &db.Review{}
	}

	if in.ReviewFile.Review.Project == nil {
		in.ReviewFile.Review.Project = &db.Project{}
	}

	// Check if all FKs are provided. Fill them into the main struct rels

	if in.ReviewFileID != 0 {
		in.ReviewFile.ID = in.ReviewFileID
	}

	// Fetch the relation. It creates if the FKs are provided it fetch from DB by PKs. Else it creates new one.
	{
		rel, relatedCleaner := ReviewFile(t, dbo, in.ReviewFile, WithReviewFileRelations, WithFakeReviewFile)
		in.ReviewFile = rel
		in.ReviewFileID = rel.ID
		// Fill the same relations as in ReviewFile
		in.ReviewFile.Review.Prompt = rel.Review.Project.Prompt

		cleaners = append(cleaners, relatedCleaner)
	}

	return func() {
		// Clean up related entities from the last to the first
		for i := len(cleaners) - 1; i >= 0; i-- {
			cleaners[i]()
		}
	}
}

func WithFakeIssue(t *testing.T, dbo orm.DB, in *db.Issue) Cleaner {
	if in.IssueType == "" {
		in.IssueType = cutS(gofakeit.Sentence(3), 32)
	}

	if in.ReviewID == 0 {
		in.ReviewID = gofakeit.IntRange(1, 10)
	}

	if in.Title == "" {
		in.Title = cutS(gofakeit.Sentence(10), 255)
	}

	if in.Severity == "" {
		in.Severity = cutS(gofakeit.Word(), 16)
	}

	if in.Description == "" {
		in.Description = cutS(gofakeit.Sentence(10), 0)
	}

	if in.Content == "" {
		in.Content = cutS(gofakeit.Sentence(10), 0)
	}

	if in.File == "" {
		in.File = cutS(gofakeit.Sentence(10), 255)
	}

	if in.Lines == "" {
		in.Lines = cutS(gofakeit.Sentence(10), 255)
	}

	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now()
	}

	if in.StatusID == 0 {
		in.StatusID = 1
	}

	return emptyClean
}

type ReviewFileOpFunc func(t *testing.T, dbo orm.DB, in *db.ReviewFile) Cleaner

func ReviewFile(t *testing.T, dbo orm.DB, in *db.ReviewFile, ops ...ReviewFileOpFunc) (*db.ReviewFile, Cleaner) {
	repo := db.NewReviewRepo(dbo)
	var cleaners []Cleaner

	// Fill the incoming entity
	if in == nil {
		in = &db.ReviewFile{}
	}

	// Check if PKs are provided
	if in.ID != 0 {
		// Fetch the entity by PK
		reviewFile, err := repo.ReviewFileByID(t.Context(), in.ID, repo.FullReviewFile())
		if err != nil {
			t.Fatal(err)
		}

		// We must find the entity by PK
		if reviewFile == nil {
			t.Fatalf("the entity ReviewFile is not found by provided PKs ID=%v", in.ID)
		}

		// Return if found without real cleanup
		return reviewFile, emptyClean
	}

	for _, op := range ops {
		if cl := op(t, dbo, in); cl != nil {
			cleaners = append(cleaners, cl)
		}
	}

	// Create the main entity
	reviewFile, err := repo.AddReviewFile(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}

	return reviewFile, func() {
		if _, err := dbo.ModelContext(context.Background(), &db.ReviewFile{ID: reviewFile.ID}).WherePK().Delete(); err != nil {
			t.Fatal(err)
		}
		// Clean up related entities from the last to the first
		for i := len(cleaners) - 1; i >= 0; i-- {
			cleaners[i]()
		}
	}
}

func WithReviewFileRelations(t *testing.T, dbo orm.DB, in *db.ReviewFile) Cleaner {
	var cleaners []Cleaner

	// Prepare main relations
	if in.Review == nil {
		in.Review = &db.Review{}
	}

	// Prepare nested relations which have the same relations
	if in.Review.Project == nil {
		in.Review.Project = &db.Project{}
	}

	// Check if all FKs are provided. Fill them into the main struct rels

	if in.ReviewID != 0 {
		in.Review.ID = in.ReviewID
	}

	// Fetch the relation. It creates if the FKs are provided it fetch from DB by PKs. Else it creates new one.
	{
		rel, relatedCleaner := Review(t, dbo, in.Review, WithReviewRelations, WithFakeReview)
		in.Review = rel
		in.ReviewID = rel.ID
		// Fill the same relations as in Review
		in.Review.Prompt = rel.Project.Prompt

		cleaners = append(cleaners, relatedCleaner)
	}

	return func() {
		// Clean up related entities from the last to the first
		for i := len(cleaners) - 1; i >= 0; i-- {
			cleaners[i]()
		}
	}
}

func WithFakeReviewFile(t *testing.T, dbo orm.DB, in *db.ReviewFile) Cleaner {
	if in.ReviewType == "" {
		in.ReviewType = cutS(gofakeit.Sentence(6), 64)
	}

	if in.Content == "" {
		in.Content = cutS(gofakeit.Sentence(10), 0)
	}

	if in.TrafficLight == "" {
		in.TrafficLight = cutS(gofakeit.Sentence(3), 32)
	}

	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now()
	}

	if in.Summary == "" {
		in.Summary = cutS(gofakeit.Sentence(10), 2048)
	}

	if in.StatusID == 0 {
		in.StatusID = 1
	}

	return emptyClean
}

type ReviewOpFunc func(t *testing.T, dbo orm.DB, in *db.Review) Cleaner

func Review(t *testing.T, dbo orm.DB, in *db.Review, ops ...ReviewOpFunc) (*db.Review, Cleaner) {
	repo := db.NewReviewRepo(dbo)
	var cleaners []Cleaner

	// Fill the incoming entity
	if in == nil {
		in = &db.Review{}
	}

	// Check if PKs are provided
	if in.ID != 0 {
		// Fetch the entity by PK
		review, err := repo.ReviewByID(t.Context(), in.ID, repo.FullReview())
		if err != nil {
			t.Fatal(err)
		}

		// We must find the entity by PK
		if review == nil {
			t.Fatalf("the entity Review is not found by provided PKs ID=%v", in.ID)
		}

		// Return if found without real cleanup
		return review, emptyClean
	}

	for _, op := range ops {
		if cl := op(t, dbo, in); cl != nil {
			cleaners = append(cleaners, cl)
		}
	}

	// Create the main entity
	review, err := repo.AddReview(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}

	return review, func() {
		if _, err := dbo.ModelContext(context.Background(), &db.Review{ID: review.ID}).WherePK().Delete(); err != nil {
			t.Fatal(err)
		}
		// Clean up related entities from the last to the first
		for i := len(cleaners) - 1; i >= 0; i-- {
			cleaners[i]()
		}
	}
}

func WithReviewRelations(t *testing.T, dbo orm.DB, in *db.Review) Cleaner {
	var cleaners []Cleaner

	// Prepare main relations
	if in.Project == nil {
		in.Project = &db.Project{}
	}

	if in.Prompt == nil {
		in.Prompt = &db.Prompt{}
	}

	// Check if all FKs are provided. Fill them into the main struct rels

	if in.ProjectID != 0 {
		in.Project.ID = in.ProjectID
	}

	if in.PromptID != 0 {
		in.Prompt.ID = in.PromptID
	}

	// Inject relation IDs into relations which have the same relations
	in.Project.PromptID = in.PromptID
	in.Project.Prompt = in.Prompt
	// Fetch the relation. It creates if the FKs are provided it fetch from DB by PKs. Else it creates new one.
	{
		rel, relatedCleaner := Project(t, dbo, in.Project, WithProjectRelations, WithFakeProject)
		in.Project = rel
		in.ProjectID = rel.ID
		// Fill the same relations as in Project
		in.Prompt = rel.Prompt

		cleaners = append(cleaners, relatedCleaner)
	}

	// Fetch the relation. It creates if the FKs are provided it fetch from DB by PKs. Else it creates new one.
	{
		rel, relatedCleaner := Prompt(t, dbo, in.Prompt, WithFakePrompt)
		in.Prompt = rel
		in.PromptID = rel.ID

		cleaners = append(cleaners, relatedCleaner)
	}

	return func() {
		// Clean up related entities from the last to the first
		for i := len(cleaners) - 1; i >= 0; i-- {
			cleaners[i]()
		}
	}
}

func WithFakeReview(t *testing.T, dbo orm.DB, in *db.Review) Cleaner {
	if in.Title == "" {
		in.Title = cutS(gofakeit.Sentence(10), 255)
	}

	if in.Description == "" {
		in.Description = cutS(gofakeit.Sentence(10), 2048)
	}

	if in.ExternalID == "" {
		in.ExternalID = cutS(gofakeit.Sentence(3), 32)
	}

	if in.TrafficLight == "" {
		in.TrafficLight = cutS(gofakeit.Sentence(3), 32)
	}

	if in.CommitHash == "" {
		in.CommitHash = cutS(gofakeit.Sentence(4), 40)
	}

	if in.SourceBranch == "" {
		in.SourceBranch = cutS(gofakeit.Sentence(10), 255)
	}

	if in.TargetBranch == "" {
		in.TargetBranch = cutS(gofakeit.Sentence(10), 255)
	}

	if in.Author == "" {
		in.Author = cutS(gofakeit.Sentence(10), 255)
	}

	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now()
	}

	if in.DurationMS == 0 {
		in.DurationMS = gofakeit.IntRange(1, 10)
	}

	if in.StatusID == 0 {
		in.StatusID = 1
	}

	return emptyClean
}
