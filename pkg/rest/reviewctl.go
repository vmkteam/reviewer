package rest

import (
	"encoding/json"
	"net/http"
	"strings"

	"reviewsrv/pkg/db"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// ReviewctlConfig is the resolved run configuration returned to reviewctl over the
// internal /v1/reviewctl/rpc/ path. It carries the real token (env vars still take
// priority on the client) — this path is CI-internal and must not be exposed.
type ReviewctlConfig struct {
	RunnerProfileID int                    `json:"runnerProfileId"`
	Title           string                 `json:"title"`
	Runner          string                 `json:"runner"`
	Model           string                 `json:"model"`
	Effort          string                 `json:"effort"`
	APIProvider     string                 `json:"apiProvider"`
	APIBaseURL      string                 `json:"apiBaseURL"`
	Token           string                 `json:"token"`
	Params          db.RunnerProfileParams `json:"params"`
}

func newReviewctlConfig(rp *db.RunnerProfile) ReviewctlConfig {
	return ReviewctlConfig{
		RunnerProfileID: rp.ID,
		Title:           rp.Title,
		Runner:          rp.Runner,
		Model:           derefString(rp.Model),
		Effort:          derefString(rp.Effort),
		APIProvider:     derefString(rp.APIProvider),
		APIBaseURL:      derefString(rp.APIBaseURL),
		Token:           derefString(rp.Token),
		Params:          rp.Params,
	}
}

// JSON-RPC 2.0 envelope types for the reviewctl internal API.
type rpcRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	ID     json.RawMessage `json:"id"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
	ID      json.RawMessage `json:"id"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type projectKeyParams struct {
	ProjectKey string `json:"projectKey"`
}

// ReviewctlRPC is a minimal JSON-RPC 2.0 endpoint for reviewctl. It exposes
// ReviewConfig(projectKey) -> ReviewctlConfig and Prompt(projectKey) -> string,
// replacing the standalone /v1/prompt/ endpoint and delivering the resolved
// runner profile so CI needs only the image + credentials.
func (h *Handler) ReviewctlRPC(c echo.Context) error {
	var req rpcRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusOK, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
	}

	var p projectKeyParams
	_ = json.Unmarshal(req.Params, &p)

	ctx := c.Request().Context()
	switch strings.TrimPrefix(req.Method, "reviewctl.") {
	case "ReviewConfig":
		if _, err := uuid.Parse(p.ProjectKey); err != nil {
			return rpcReply(c, req.ID, nil, &rpcError{Code: -32602, Message: "invalid project key"})
		}
		rp, err := h.pm.RunnerProfile(ctx, p.ProjectKey)
		if err != nil {
			return rpcReply(c, req.ID, nil, &rpcError{Code: -32603, Message: err.Error()})
		}
		if rp == nil {
			return rpcReply(c, req.ID, nil, &rpcError{Code: -32004, Message: "no runner profile for project (and no default)"})
		}
		return rpcReply(c, req.ID, newReviewctlConfig(rp), nil)

	case "Prompt":
		if _, err := uuid.Parse(p.ProjectKey); err != nil {
			return rpcReply(c, req.ID, nil, &rpcError{Code: -32602, Message: "invalid project key"})
		}
		prompt, err := h.pm.Prompt(ctx, p.ProjectKey)
		if err != nil {
			return rpcReply(c, req.ID, nil, &rpcError{Code: -32603, Message: err.Error()})
		}
		return rpcReply(c, req.ID, prompt, nil)

	default:
		return rpcReply(c, req.ID, nil, &rpcError{Code: -32601, Message: "method not found"})
	}
}

func rpcReply(c echo.Context, id json.RawMessage, result any, rerr *rpcError) error {
	return c.JSON(http.StatusOK, rpcResponse{JSONRPC: "2.0", Result: result, Error: rerr, ID: id})
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
