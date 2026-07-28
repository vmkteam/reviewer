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

// TrackerConfig is the project task-tracker access config: the base URL that
// scopes the direct runner's http_fetch tool and the auth token reviewctl hands
// to runners out-of-band (http_fetch header / $REVIEW_TRACKER_TOKEN env), so
// the token never appears in the assembled prompt. CI-internal path only.
type TrackerConfig struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

// ReviewSetup is the resolved multi-review panel returned to reviewctl: the
// primary runner, the additional panel members and the optional judge. The full
// run set is [Primary] + Panel; a non-nil Judge is the multi-review trigger (nil →
// single review via Primary, today's flow). Every Config carries its real token
// (CI-internal path), so this must never be exposed on the public RPC.
// Tracker is the project's task-tracker access config (nil = none configured).
type ReviewSetup struct {
	Primary *Config        `json:"primary"`
	Panel   []*Config      `json:"panel"`
	Judge   *Config        `json:"judge"`
	Tracker *TrackerConfig `json:"tracker"`
}

func newConfig(rp *reviewer.RunnerProfile) *Config {
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

	rs, err := s.pm.ReviewProfiles(ctx, projectKey)
	if err != nil {
		return nil, newInternalError(err)
	}
	if rs == nil || rs.Primary == nil {
		return nil, ErrNoRunnerProfile
	}

	setup := &ReviewSetup{Primary: newConfig(rs.Primary)}
	for _, rp := range rs.Panel {
		setup.Panel = append(setup.Panel, newConfig(rp))
	}
	if rs.Judge != nil {
		setup.Judge = newConfig(rs.Judge)
	}
	if rs.Tracker != nil {
		setup.Tracker = &TrackerConfig{URL: rs.Tracker.URL, Token: derefString(rs.Tracker.AuthToken)}
	}

	return setup, nil
}

// Prompt assembles and returns the review prompt for a project key. New
// reviewctl clients pass tokenEnv=true and get the tracker section referencing
// the $REVIEW_TRACKER_TOKEN env var (the secret travels out-of-band via
// ReviewConfig). Legacy clients omit the flag and keep the historical prompt
// with the real tracker token substituted in, so old CI images stay working.
//
//zenrpc:projectKey project key (UUID)
//zenrpc:tokenEnv=false substitute the $REVIEW_TRACKER_TOKEN env reference instead of the real tracker token
//zenrpc:return string
//zenrpc:400 invalid project key
//zenrpc:500 internal error
func (s Service) Prompt(ctx context.Context, projectKey string, tokenEnv *bool) (string, error) {
	if _, err := uuid.Parse(projectKey); err != nil {
		return "", ErrInvalidProjectKey
	}

	prompt, err := s.pm.Prompt(ctx, projectKey, tokenEnv != nil && *tokenEnv)
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
