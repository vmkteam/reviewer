package ctl

import "testing"

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
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
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
			if got := tt.cfg.HasGitLab(); got != tt.want {
				t.Errorf("HasGitLab() = %v, want %v", got, tt.want)
			}
		})
	}
}
