package reviewctl

import (
	"context"

	"reviewsrv/pkg/db"
	"reviewsrv/pkg/reviewer"

	"github.com/google/uuid"
	"github.com/vmkteam/zenrpc/v2"
)

// Service is the internal JSON-RPC service that reviewctl (CI) calls to fetch its
// resolved runner profile and the assembled review prompt. It is mounted only on
// the network-internal /v1/reviewctl/rpc/ path and must never be registered on the
// public RPC server, because ReviewConfig returns the real profile token.
type Service struct {
	pm *reviewer.ProjectManager
	zenrpc.Service
}

// NewService creates the reviewctl JSON-RPC service.
func NewService(dbc db.DB) *Service {
	return &Service{pm: reviewer.NewProjectManager(dbc)}
}

// RunnerProfileParams mirrors db.RunnerProfileParams as a service-local type so the
// generated client emits a clean RunnerProfileParams model — a db-qualified type
// name would generate DbRunnerProfileParams.
type RunnerProfileParams struct {
	AllowDangerousPermissions bool `json:"allowDangerousPermissions"`
}

// Config is the resolved run configuration returned to reviewctl. It carries the
// real token (env vars still take priority on the client) — this path is
// CI-internal and must not be exposed publicly.
type Config struct {
	RunnerProfileID int                 `json:"runnerProfileId"`
	Title           string              `json:"title"`
	Runner          string              `json:"runner"`
	Model           string              `json:"model"`
	Effort          string              `json:"effort"`
	APIProvider     string              `json:"apiProvider"`
	APIBaseURL      string              `json:"apiBaseURL"`
	Token           string              `json:"token"`
	Params          RunnerProfileParams `json:"params"`
}

func newConfig(rp *db.RunnerProfile) *Config {
	return &Config{
		RunnerProfileID: rp.ID,
		Title:           rp.Title,
		Runner:          rp.Runner,
		Model:           derefString(rp.Model),
		Effort:          derefString(rp.Effort),
		APIProvider:     derefString(rp.APIProvider),
		APIBaseURL:      derefString(rp.APIBaseURL),
		Token:           derefString(rp.Token),
		Params:          RunnerProfileParams{AllowDangerousPermissions: rp.Params.AllowDangerousPermissions},
	}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ReviewConfig resolves the runner profile for a project key — the project's pinned
// profile or the default — and returns the run config including the real token.
//
//zenrpc:projectKey project key (UUID)
//zenrpc:return Config
//zenrpc:400 invalid project key
//zenrpc:404 no runner profile for project (and no default)
//zenrpc:500 internal error
func (s Service) ReviewConfig(ctx context.Context, projectKey string) (*Config, error) {
	if _, err := uuid.Parse(projectKey); err != nil {
		return nil, ErrInvalidProjectKey
	}

	rp, err := s.pm.RunnerProfile(ctx, projectKey)
	if err != nil {
		return nil, newInternalError(err)
	}
	if rp == nil {
		return nil, ErrNoRunnerProfile
	}

	return newConfig(rp), nil
}

// Prompt assembles and returns the review prompt for a project key.
//
//zenrpc:projectKey project key (UUID)
//zenrpc:return string
//zenrpc:400 invalid project key
//zenrpc:500 internal error
func (s Service) Prompt(ctx context.Context, projectKey string) (string, error) {
	if _, err := uuid.Parse(projectKey); err != nil {
		return "", ErrInvalidProjectKey
	}

	prompt, err := s.pm.Prompt(ctx, projectKey)
	if err != nil {
		return "", newInternalError(err)
	}

	return prompt, nil
}
