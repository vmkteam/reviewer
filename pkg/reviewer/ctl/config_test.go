package ctl

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"reviewsrv/pkg/reviewer/runner"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		cmd     string
		wantErr bool
	}{
		{"empty key", Config{URL: "http://x"}, "review", true},
		{"empty url", Config{Key: "k"}, "review", true},
		{"valid review", Config{Key: "k", URL: "http://x"}, "review", false},
		{"valid upload", Config{Key: "k", URL: "http://x"}, "upload", false},
		{"comment no id", Config{Key: "k", URL: "http://x"}, "comment", true},
		{"comment with id", Config{Key: "k", URL: "http://x", ReviewID: 1}, "comment", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate(tt.cmd)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigResolveDefaults(t *testing.T) {
	tests := []struct {
		name       string
		runner     string
		provider   string
		model      string
		effort     string
		wantModel  string
		wantEffort string
	}{
		{"claude empty model defaults to opus", runner.RunnerClaude, "", "", "", "opus", ""},
		{"empty runner defaults to claude+opus", "", "", "", "", "opus", ""},
		{"claude with explicit model preserved", runner.RunnerClaude, "", "sonnet", "", "sonnet", ""},
		{"claude effort preserved, no default", runner.RunnerClaude, "", "", "high", "opus", "high"},
		{"opencode empty model stays empty", runner.RunnerOpenCode, "", "", "", "", ""},
		{"opencode explicit model preserved", runner.RunnerOpenCode, "", "anthropic/claude-opus-4", "", "anthropic/claude-opus-4", ""},
		{"direct+anthropic pins model and xhigh effort", runner.RunnerDirect, "anthropic", "", "", "claude-opus-4-8", "xhigh"},
		{"direct+anthropic explicit effort preserved", runner.RunnerDirect, "anthropic", "", "max", "claude-opus-4-8", "max"},
		{"direct+deepseek leaves model/effort untouched", runner.RunnerDirect, "deepseek", "", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Config{Runner: tt.runner, APIProvider: tt.provider, Model: tt.model, Effort: tt.effort}
			c.ResolveDefaults()
			assert.Equal(t, tt.wantModel, c.Model)
			assert.Equal(t, tt.wantEffort, c.Effort)
		})
	}
}

func TestConfigHasGitLab(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{"all set", Config{GitLabToken: "t", GitLabURL: "u", MRIID: "1", ProjectID: "2"}, true},
		{"no token", Config{GitLabURL: "u", MRIID: "1", ProjectID: "2"}, false},
		{"no url", Config{GitLabToken: "t", MRIID: "1", ProjectID: "2"}, false},
		{"no mriid", Config{GitLabToken: "t", GitLabURL: "u", ProjectID: "2"}, false},
		{"no project", Config{GitLabToken: "t", GitLabURL: "u", MRIID: "1"}, false},
		{"empty", Config{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cfg.HasGitLab())
		})
	}
}
