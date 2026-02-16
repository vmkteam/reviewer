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

type ProjectOpFunc func(t *testing.T, dbo orm.DB, in *db.Project) Cleaner

func Project(t *testing.T, dbo orm.DB, in *db.Project, ops ...ProjectOpFunc) (*db.Project, Cleaner) {
	repo := db.NewProjectRepo(dbo)
	var cleaners []Cleaner

	// Fill the incoming entity
	if in == nil {
		in = &db.Project{}
	}

	// Check if PKs are provided
	if in.ID != 0 {
		// Fetch the entity by PK
		project, err := repo.ProjectByID(t.Context(), in.ID, repo.FullProject())
		if err != nil {
			t.Fatal(err)
		}

		// We must find the entity by PK
		if project == nil {
			t.Fatalf("the entity Project is not found by provided PKs ID=%v", in.ID)
		}

		// Return if found without real cleanup
		return project, emptyClean
	}

	for _, op := range ops {
		if cl := op(t, dbo, in); cl != nil {
			cleaners = append(cleaners, cl)
		}
	}

	// Create the main entity
	project, err := repo.AddProject(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}

	return project, func() {
		if _, err := dbo.ModelContext(context.Background(), &db.Project{ID: project.ID}).WherePK().Delete(); err != nil {
			t.Fatal(err)
		}
		// Clean up related entities from the last to the first
		for i := len(cleaners) - 1; i >= 0; i-- {
			cleaners[i]()
		}
	}
}

func WithProjectRelations(t *testing.T, dbo orm.DB, in *db.Project) Cleaner {
	var cleaners []Cleaner

	// Prepare main relations
	if in.Prompt == nil {
		in.Prompt = &db.Prompt{}
	}

	// Check if all FKs are provided. Fill them into the main struct rels

	if in.PromptID != 0 {
		in.Prompt.ID = in.PromptID
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

func WithFakeProject(t *testing.T, dbo orm.DB, in *db.Project) Cleaner {
	if in.Title == "" {
		in.Title = cutS(gofakeit.Sentence(10), 255)
	}

	if in.VcsURL == "" {
		in.VcsURL = cutS(gofakeit.Sentence(10), 255)
	}

	if in.Language == "" {
		in.Language = cutS(gofakeit.Sentence(3), 32)
	}

	if in.ProjectKey == "" {
		in.ProjectKey = gofakeit.UUID()
	}

	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now()
	}

	if in.StatusID == 0 {
		in.StatusID = 1
	}

	return emptyClean
}

type PromptOpFunc func(t *testing.T, dbo orm.DB, in *db.Prompt) Cleaner

func Prompt(t *testing.T, dbo orm.DB, in *db.Prompt, ops ...PromptOpFunc) (*db.Prompt, Cleaner) {
	repo := db.NewProjectRepo(dbo)
	var cleaners []Cleaner

	// Fill the incoming entity
	if in == nil {
		in = &db.Prompt{}
	}

	// Check if PKs are provided
	if in.ID != 0 {
		// Fetch the entity by PK
		prompt, err := repo.PromptByID(t.Context(), in.ID, repo.FullPrompt())
		if err != nil {
			t.Fatal(err)
		}

		// We must find the entity by PK
		if prompt == nil {
			t.Fatalf("the entity Prompt is not found by provided PKs ID=%v", in.ID)
		}

		// Return if found without real cleanup
		return prompt, emptyClean
	}

	for _, op := range ops {
		if cl := op(t, dbo, in); cl != nil {
			cleaners = append(cleaners, cl)
		}
	}

	// Create the main entity
	prompt, err := repo.AddPrompt(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}

	return prompt, func() {
		if _, err := dbo.ModelContext(context.Background(), &db.Prompt{ID: prompt.ID}).WherePK().Delete(); err != nil {
			t.Fatal(err)
		}
		// Clean up related entities from the last to the first
		for i := len(cleaners) - 1; i >= 0; i-- {
			cleaners[i]()
		}
	}
}

func WithFakePrompt(t *testing.T, dbo orm.DB, in *db.Prompt) Cleaner {
	if in.Title == "" {
		in.Title = cutS(gofakeit.Sentence(10), 255)
	}

	if in.Common == "" {
		in.Common = cutS(gofakeit.Sentence(10), 0)
	}

	if in.Architecture == "" {
		in.Architecture = cutS(gofakeit.Sentence(10), 0)
	}

	if in.Code == "" {
		in.Code = cutS(gofakeit.Sentence(10), 0)
	}

	if in.Security == "" {
		in.Security = cutS(gofakeit.Sentence(10), 0)
	}

	if in.Tests == "" {
		in.Tests = cutS(gofakeit.Sentence(10), 0)
	}

	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now()
	}

	if in.StatusID == 0 {
		in.StatusID = 1
	}

	return emptyClean
}

type SlackChannelOpFunc func(t *testing.T, dbo orm.DB, in *db.SlackChannel) Cleaner

func SlackChannel(t *testing.T, dbo orm.DB, in *db.SlackChannel, ops ...SlackChannelOpFunc) (*db.SlackChannel, Cleaner) {
	repo := db.NewProjectRepo(dbo)
	var cleaners []Cleaner

	// Fill the incoming entity
	if in == nil {
		in = &db.SlackChannel{}
	}

	// Check if PKs are provided
	if in.ID != 0 {
		// Fetch the entity by PK
		slackChannel, err := repo.SlackChannelByID(t.Context(), in.ID, repo.FullSlackChannel())
		if err != nil {
			t.Fatal(err)
		}

		// We must find the entity by PK
		if slackChannel == nil {
			t.Fatalf("the entity SlackChannel is not found by provided PKs ID=%v", in.ID)
		}

		// Return if found without real cleanup
		return slackChannel, emptyClean
	}

	for _, op := range ops {
		if cl := op(t, dbo, in); cl != nil {
			cleaners = append(cleaners, cl)
		}
	}

	// Create the main entity
	slackChannel, err := repo.AddSlackChannel(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}

	return slackChannel, func() {
		if _, err := dbo.ModelContext(context.Background(), &db.SlackChannel{ID: slackChannel.ID}).WherePK().Delete(); err != nil {
			t.Fatal(err)
		}
		// Clean up related entities from the last to the first
		for i := len(cleaners) - 1; i >= 0; i-- {
			cleaners[i]()
		}
	}
}

func WithFakeSlackChannel(t *testing.T, dbo orm.DB, in *db.SlackChannel) Cleaner {
	if in.Title == "" {
		in.Title = cutS(gofakeit.Sentence(10), 255)
	}

	if in.Channel == "" {
		in.Channel = cutS(gofakeit.Sentence(10), 255)
	}

	if in.WebhookURL == "" {
		in.WebhookURL = cutS(gofakeit.Sentence(10), 1024)
	}

	if in.StatusID == 0 {
		in.StatusID = 1
	}

	return emptyClean
}

type TaskTrackerOpFunc func(t *testing.T, dbo orm.DB, in *db.TaskTracker) Cleaner

func TaskTracker(t *testing.T, dbo orm.DB, in *db.TaskTracker, ops ...TaskTrackerOpFunc) (*db.TaskTracker, Cleaner) {
	repo := db.NewProjectRepo(dbo)
	var cleaners []Cleaner

	// Fill the incoming entity
	if in == nil {
		in = &db.TaskTracker{}
	}

	// Check if PKs are provided
	if in.ID != 0 {
		// Fetch the entity by PK
		taskTracker, err := repo.TaskTrackerByID(t.Context(), in.ID, repo.FullTaskTracker())
		if err != nil {
			t.Fatal(err)
		}

		// We must find the entity by PK
		if taskTracker == nil {
			t.Fatalf("the entity TaskTracker is not found by provided PKs ID=%v", in.ID)
		}

		// Return if found without real cleanup
		return taskTracker, emptyClean
	}

	for _, op := range ops {
		if cl := op(t, dbo, in); cl != nil {
			cleaners = append(cleaners, cl)
		}
	}

	// Create the main entity
	taskTracker, err := repo.AddTaskTracker(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}

	return taskTracker, func() {
		if _, err := dbo.ModelContext(context.Background(), &db.TaskTracker{ID: taskTracker.ID}).WherePK().Delete(); err != nil {
			t.Fatal(err)
		}
		// Clean up related entities from the last to the first
		for i := len(cleaners) - 1; i >= 0; i-- {
			cleaners[i]()
		}
	}
}

func WithFakeTaskTracker(t *testing.T, dbo orm.DB, in *db.TaskTracker) Cleaner {
	if in.Title == "" {
		in.Title = cutS(gofakeit.Sentence(10), 255)
	}

	if in.AuthToken == "" {
		in.AuthToken = cutS(gofakeit.Sentence(10), 255)
	}

	if in.FetchPrompt == "" {
		in.FetchPrompt = cutS(gofakeit.Sentence(10), 0)
	}

	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now()
	}

	if in.StatusID == 0 {
		in.StatusID = 1
	}

	return emptyClean
}
