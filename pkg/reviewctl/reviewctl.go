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

// ReviewSetup is the resolved multi-review panel returned to reviewctl: the
// primary runner, the additional panel members and the optional judge. The full
// run set is [Primary] + Panel; a non-nil Judge is the multi-review trigger (nil →
// single review via Primary, today's flow). Every Config carries its real token
// (CI-internal path), so this must never be exposed on the public RPC.
type ReviewSetup struct {
	Primary *Config   `json:"primary"`
	Panel   []*Config `json:"panel"`
	Judge   *Config   `json:"judge"`
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

// ReviewConfig resolves the multi-review panel for a project key: the primary
// runner (pinned profile or default), the additional panel members
// (runnerProfileIds) and the optional judge (judgeRunnerProfileId). Each config
// includes the real token. A nil judge means single review via the primary.
//
//zenrpc:projectKey project key (UUID)
//zenrpc:return ReviewSetup
//zenrpc:400 invalid project key
//zenrpc:404 no runner profile for project (and no default)
//zenrpc:500 internal error
func (s Service) ReviewConfig(ctx context.Context, projectKey string) (*ReviewSetup, error) {
	if _, err := uuid.Parse(projectKey); err != nil {
		return nil, ErrInvalidProjectKey
	}

	primary, panel, judge, err := s.pm.ReviewProfiles(ctx, projectKey)
	if err != nil {
		return nil, newInternalError(err)
	}
	if primary == nil {
		return nil, ErrNoRunnerProfile
	}

	setup := &ReviewSetup{Primary: newConfig(primary)}
	for _, rp := range panel {
		setup.Panel = append(setup.Panel, newConfig(rp))
	}
	if judge != nil {
		setup.Judge = newConfig(judge)
	}

	return setup, nil
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

// FusionPrompt returns the built-in judge/synthesizer prompt for multi-review.
// The algorithm is project-agnostic, so the same built-in is served for every
// project; the projectKey is validated for a consistent contract with Prompt and
// to leave room for a per-project override later.
//
//zenrpc:projectKey project key (UUID)
//zenrpc:return string
//zenrpc:400 invalid project key
func (s Service) FusionPrompt(ctx context.Context, projectKey string) (string, error) {
	if _, err := uuid.Parse(projectKey); err != nil {
		return "", ErrInvalidProjectKey
	}

	return reviewer.FusionPrompt, nil
}
