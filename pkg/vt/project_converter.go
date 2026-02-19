package vt

import (
	"reviewsrv/pkg/db"
)

func NewProject(in *db.Project) *Project {
	if in == nil {
		return nil
	}

	project := &Project{
		ID:             in.ID,
		Title:          in.Title,
		VcsURL:         in.VcsURL,
		Language:       in.Language,
		ProjectKey:     in.ProjectKey,
		PromptID:       in.PromptID,
		TaskTrackerID:  in.TaskTrackerID,
		SlackChannelID: in.SlackChannelID,
		StatusID:       in.StatusID,
		Instructions:   in.Instructions,

		Prompt:       NewPromptSummary(in.Prompt),
		TaskTracker:  NewTaskTrackerSummary(in.TaskTracker),
		SlackChannel: NewSlackChannelSummary(in.SlackChannel),
		Status:       NewStatus(in.StatusID),
	}

	return project
}

func NewProjectSummary(in *db.Project) *ProjectSummary {
	if in == nil {
		return nil
	}

	return &ProjectSummary{
		ID:             in.ID,
		Title:          in.Title,
		VcsURL:         in.VcsURL,
		Language:       in.Language,
		ProjectKey:     in.ProjectKey,
		PromptID:       in.PromptID,
		TaskTrackerID:  in.TaskTrackerID,
		SlackChannelID: in.SlackChannelID,

		Prompt:       NewPromptSummary(in.Prompt),
		TaskTracker:  NewTaskTrackerSummary(in.TaskTracker),
		SlackChannel: NewSlackChannelSummary(in.SlackChannel),
		Status:       NewStatus(in.StatusID),
	}
}

func NewPrompt(in *db.Prompt) *Prompt {
	if in == nil {
		return nil
	}

	prompt := &Prompt{
		ID:           in.ID,
		Title:        in.Title,
		Common:       in.Common,
		Architecture: in.Architecture,
		Code:         in.Code,
		Security:     in.Security,
		Tests:        in.Tests,
		StatusID:     in.StatusID,

		Status: NewStatus(in.StatusID),
	}

	return prompt
}

func NewPromptSummary(in *db.Prompt) *PromptSummary {
	if in == nil {
		return nil
	}

	return &PromptSummary{
		ID:           in.ID,
		Title:        in.Title,
		Common:       in.Common,
		Architecture: in.Architecture,
		Code:         in.Code,
		Security:     in.Security,
		Tests:        in.Tests,

		Status: NewStatus(in.StatusID),
	}
}

func NewSlackChannel(in *db.SlackChannel) *SlackChannel {
	if in == nil {
		return nil
	}

	slackChannel := &SlackChannel{
		ID:         in.ID,
		Title:      in.Title,
		Channel:    in.Channel,
		WebhookURL: in.WebhookURL,
		StatusID:   in.StatusID,

		Status: NewStatus(in.StatusID),
	}

	return slackChannel
}

func NewSlackChannelSummary(in *db.SlackChannel) *SlackChannelSummary {
	if in == nil {
		return nil
	}

	return &SlackChannelSummary{
		ID:         in.ID,
		Title:      in.Title,
		Channel:    in.Channel,
		WebhookURL: in.WebhookURL,

		Status: NewStatus(in.StatusID),
	}
}

func NewTaskTracker(in *db.TaskTracker) *TaskTracker {
	if in == nil {
		return nil
	}

	taskTracker := &TaskTracker{
		ID:          in.ID,
		Title:       in.Title,
		URL:         in.URL,
		AuthToken:   in.AuthToken,
		FetchPrompt: in.FetchPrompt,
		StatusID:    in.StatusID,

		Status: NewStatus(in.StatusID),
	}

	return taskTracker
}

func NewTaskTrackerSummary(in *db.TaskTracker) *TaskTrackerSummary {
	if in == nil {
		return nil
	}

	return &TaskTrackerSummary{
		ID:          in.ID,
		Title:       in.Title,
		URL:         in.URL,
		AuthToken:   in.AuthToken,
		FetchPrompt: in.FetchPrompt,

		Status: NewStatus(in.StatusID),
	}
}
